package jsoniter

import (
	"encoding/json"
	"io"
	"reflect"
	"sync"
	"unsafe"

	"github.com/modern-go/concurrent"
	"github.com/modern-go/reflect2"
)

// Config customize how the API should behave.
// The API is created from Config by Froze.
type Config struct {
	IndentionStep                 int
	MarshalFloatWith6Digits       bool
	EscapeHTML                    bool
	SortMapKeys                   bool
	UseNumber                     bool
	DisallowUnknownFields         bool
	TagKey                        string
	OnlyTaggedField               bool
	ValidateJsonRawMessage        bool
	ObjectFieldMustBeSimpleString bool
	CaseSensitive                 bool
}

// A lot of people feel like use it like `encoding/json` ,jsoniter.NewIJSON().Marshal()...
func NewIJSON() API {
	return ConfigCompatibleWithStandardLibrary
}

// API the public interface of this package.
// Primary Marshal and Unmarshal.
type API interface {
	IteratorPool
	StreamPool
	MarshalToString(v interface{}) (string, error)
	Marshal(v interface{}) ([]byte, error)
	MarshalIndent(v interface{}, prefix, indent string) ([]byte, error)
	UnmarshalFromString(str string, v interface{}) error
	Unmarshal(data []byte, v interface{}) error
	Get(data []byte, path ...interface{}) Any
	NewEncoder(writer io.Writer) *Encoder
	NewDecoder(reader io.Reader) *Decoder
	Valid(data []byte) bool
	RegisterExtension(extension Extension)
	DecoderOf(typ reflect2.Type) ValDecoder
	EncoderOf(typ reflect2.Type) ValEncoder
}

// ConfigDefault the default API
var ConfigDefault = Config{
	EscapeHTML: true,
}.Froze()

// ConfigCompatibleWithStandardLibrary tries to be 100% compatible with standard library behavior
var ConfigCompatibleWithStandardLibrary = Config{
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
}.Froze()

// ConfigFastest marshals float with only 6 digits precision
var ConfigFastest = Config{
	EscapeHTML:                    false,
	MarshalFloatWith6Digits:       true, // will lose precession
	ObjectFieldMustBeSimpleString: true, // do not unescape object field
}.Froze()

type frozenConfig struct {
	configBeforeFrozen            Config
	sortMapKeys                   bool
	indentionStep                 int
	objectFieldMustBeSimpleString bool
	onlyTaggedField               bool
	disallowUnknownFields         bool
	decoderCache                  *concurrent.Map
	encoderCache                  *concurrent.Map
	encoderExtension              Extension
	decoderExtension              Extension
	extraExtensions               []Extension
	streamPool                    *sync.Pool
	iteratorPool                  *sync.Pool
	caseSensitive                 bool
}

func (cfg *frozenConfig) initCache() {
	cfg.decoderCache = concurrent.NewMap() // 解码缓存
	cfg.encoderCache = concurrent.NewMap() // 编码缓存
}

// 添加到缓存
func (cfg *frozenConfig) addDecoderToCache(cacheKey uintptr, decoder ValDecoder) {
	cfg.decoderCache.Store(cacheKey, decoder)
}

// 添加到缓存
func (cfg *frozenConfig) addEncoderToCache(cacheKey uintptr, encoder ValEncoder) {
	cfg.encoderCache.Store(cacheKey, encoder)
}

// 获取缓存中解码器
func (cfg *frozenConfig) getDecoderFromCache(cacheKey uintptr) ValDecoder {
	decoder, found := cfg.decoderCache.Load(cacheKey)
	if found {
		return decoder.(ValDecoder)
	}
	return nil
}

// 获取缓存中编码器
func (cfg *frozenConfig) getEncoderFromCache(cacheKey uintptr) ValEncoder {
	encoder, found := cfg.encoderCache.Load(cacheKey)
	if found {
		return encoder.(ValEncoder)
	}
	return nil
}

// 配置缓存
var cfgCache = concurrent.NewMap()

func getFrozenConfigFromCache(cfg Config) *frozenConfig {
	obj, found := cfgCache.Load(cfg)
	if found {
		return obj.(*frozenConfig)
	}
	return nil
}

func addFrozenConfigToCache(cfg Config, frozenConfig *frozenConfig) {
	cfgCache.Store(cfg, frozenConfig)
}

// Froze forge API from config
func (cfg Config) Froze() API {
	api := &frozenConfig{
		sortMapKeys:                   cfg.SortMapKeys,
		indentionStep:                 cfg.IndentionStep,
		objectFieldMustBeSimpleString: cfg.ObjectFieldMustBeSimpleString,
		onlyTaggedField:               cfg.OnlyTaggedField,
		disallowUnknownFields:         cfg.DisallowUnknownFields,
		caseSensitive:                 cfg.CaseSensitive,
	}
	api.streamPool = &sync.Pool{ // 缓存stream  便于重复利用 减少GC压力
		New: func() interface{} {
			return NewStream(api, nil, 512)
		},
	}
	api.iteratorPool = &sync.Pool{ // 缓存iterator 便于重复利用 减少GC压力
		New: func() interface{} {
			return NewIterator(api)
		},
	}
	api.initCache() // 编码和解码本地缓存
	encoderExtension := EncoderExtension{}
	decoderExtension := DecoderExtension{}
	if cfg.MarshalFloatWith6Digits { // 添加扩展选项的内容
		api.marshalFloatWith6Digits(encoderExtension)
	}
	if cfg.EscapeHTML {
		api.escapeHTML(encoderExtension)
	}
	if cfg.UseNumber {
		api.useNumber(decoderExtension)
	}
	if cfg.ValidateJsonRawMessage {
		api.validateJsonRawMessage(encoderExtension)
	}
	api.encoderExtension = encoderExtension
	api.decoderExtension = decoderExtension
	api.configBeforeFrozen = cfg
	return api
}

// 缓冲config便于重复利用
func (cfg Config) frozeWithCacheReuse(extraExtensions []Extension) *frozenConfig {
	api := getFrozenConfigFromCache(cfg) // 获取缓存中的内容
	if api != nil {                      // 有则直接返回
		return api
	}
	api = cfg.Froze().(*frozenConfig)           // 无则重新创建新的config
	for _, extension := range extraExtensions { // 增加其他扩展选项 进行附加到config
		api.RegisterExtension(extension)
	}
	addFrozenConfigToCache(cfg, api) // 将config及其实例放置在cache中
	return api
}

// 验证json
func (cfg *frozenConfig) validateJsonRawMessage(extension EncoderExtension) {
	encoder := &funcEncoder{func(ptr unsafe.Pointer, stream *Stream) { // 编码函数
		rawMessage := *(*json.RawMessage)(ptr)         // 原生json消息
		iter := cfg.BorrowIterator([]byte(rawMessage)) // 通过获取本地cache中的iterator实例并将该iterator原有的内容重置，将其关联到新的[]byte上 便于后续的read
		iter.Read()
		if iter.Error != nil { // 获取iterator出现错误
			stream.WriteRaw("null")
		} else { // 返回iterator 并将原生json消息以string的形式写入到stream
			cfg.ReturnIterator(iter)
			stream.WriteRaw(string(rawMessage))
		}
	}, func(ptr unsafe.Pointer) bool { // 检查原生json消息是否空
		return len(*((*json.RawMessage)(ptr))) == 0
	}}
	extension[reflect2.TypeOfPtr((*json.RawMessage)(nil)).Elem()] = encoder // 保留对encoding.json的支持
	extension[reflect2.TypeOfPtr((*RawMessage)(nil)).Elem()] = encoder      // jsoniter的支持
}

func (cfg *frozenConfig) useNumber(extension DecoderExtension) {
	extension[reflect2.TypeOfPtr((*interface{})(nil)).Elem()] = &funcDecoder{func(ptr unsafe.Pointer, iter *Iterator) { // 解码
		exitingValue := *((*interface{})(ptr))
		if exitingValue != nil && reflect.TypeOf(exitingValue).Kind() == reflect.Ptr {
			iter.ReadVal(exitingValue)
			return
		}
		if iter.WhatIsNext() == NumberValue {
			*((*interface{})(ptr)) = json.Number(iter.readNumberAsString())
		} else {
			*((*interface{})(ptr)) = iter.Read()
		}
	}}
}
func (cfg *frozenConfig) getTagKey() string {
	tagKey := cfg.configBeforeFrozen.TagKey
	if tagKey == "" {
		return "json"
	}
	return tagKey
}

func (cfg *frozenConfig) RegisterExtension(extension Extension) {
	cfg.extraExtensions = append(cfg.extraExtensions, extension)
	copied := cfg.configBeforeFrozen
	cfg.configBeforeFrozen = copied
}

type lossyFloat32Encoder struct {
}

func (encoder *lossyFloat32Encoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	stream.WriteFloat32Lossy(*((*float32)(ptr)))
}

func (encoder *lossyFloat32Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*float32)(ptr)) == 0
}

type lossyFloat64Encoder struct {
}

func (encoder *lossyFloat64Encoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	stream.WriteFloat64Lossy(*((*float64)(ptr)))
}

func (encoder *lossyFloat64Encoder) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*float64)(ptr)) == 0
}

// EnableLossyFloatMarshalling keeps 10**(-6) precision
// for float variables for better performance.
func (cfg *frozenConfig) marshalFloatWith6Digits(extension EncoderExtension) {
	// for better performance
	extension[reflect2.TypeOfPtr((*float32)(nil)).Elem()] = &lossyFloat32Encoder{} // 通过只需要指定Type而不是具体的实例 减少空间的占有？？？
	extension[reflect2.TypeOfPtr((*float64)(nil)).Elem()] = &lossyFloat64Encoder{}
}

type htmlEscapedStringEncoder struct {
}

func (encoder *htmlEscapedStringEncoder) Encode(ptr unsafe.Pointer, stream *Stream) {
	str := *((*string)(ptr))
	stream.WriteStringWithHTMLEscaped(str)
}

func (encoder *htmlEscapedStringEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*string)(ptr)) == ""
}

func (cfg *frozenConfig) escapeHTML(encoderExtension EncoderExtension) {
	encoderExtension[reflect2.TypeOfPtr((*string)(nil)).Elem()] = &htmlEscapedStringEncoder{}
}

func (cfg *frozenConfig) cleanDecoders() { // 清理本地缓存中的解码器
	typeDecoders = map[string]ValDecoder{}                   // 类型
	fieldDecoders = map[string]ValDecoder{}                  // 字段
	*cfg = *(cfg.configBeforeFrozen.Froze().(*frozenConfig)) // config
}

func (cfg *frozenConfig) cleanEncoders() { // 清理本地缓存中的编码器
	typeEncoders = map[string]ValEncoder{}                   // 类型
	fieldEncoders = map[string]ValEncoder{}                  // 字段
	*cfg = *(cfg.configBeforeFrozen.Froze().(*frozenConfig)) // 配置
}

func (cfg *frozenConfig) MarshalToString(v interface{}) (string, error) { // 序列化为string格式
	stream := cfg.BorrowStream(nil)
	defer cfg.ReturnStream(stream)
	stream.WriteVal(v)
	if stream.Error != nil {
		return "", stream.Error
	}
	return string(stream.Buffer()), nil
}

func (cfg *frozenConfig) Marshal(v interface{}) ([]byte, error) { // 序列化
	stream := cfg.BorrowStream(nil)
	defer cfg.ReturnStream(stream)
	stream.WriteVal(v)
	if stream.Error != nil {
		return nil, stream.Error
	}
	result := stream.Buffer() // 通过将stream对应的[]byte赋值到临时[]byte 便于stream进行回收到Pool中
	copied := make([]byte, len(result))
	copy(copied, result) // 将stream关联的[]byte 拷贝到临时[]byte中
	return copied, nil
}

func (cfg *frozenConfig) MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) { // 输出有层次的json
	if prefix != "" {
		panic("prefix is not supported")
	}
	for _, r := range indent {
		if r != ' ' {
			panic("indent can only be space")
		}
	}
	newCfg := cfg.configBeforeFrozen
	newCfg.IndentionStep = len(indent)
	return newCfg.frozeWithCacheReuse(cfg.extraExtensions).Marshal(v)
}

func (cfg *frozenConfig) UnmarshalFromString(str string, v interface{}) error {
	data := []byte(str)
	iter := cfg.BorrowIterator(data)
	defer cfg.ReturnIterator(iter)
	iter.ReadVal(v)
	c := iter.nextToken()
	if c == 0 {
		if iter.Error == io.EOF {
			return nil
		}
		return iter.Error
	}
	iter.ReportError("Unmarshal", "there are bytes left after unmarshal")
	return iter.Error
}

func (cfg *frozenConfig) Get(data []byte, path ...interface{}) Any {
	iter := cfg.BorrowIterator(data)
	defer cfg.ReturnIterator(iter)
	return locatePath(iter, path)
}

func (cfg *frozenConfig) Unmarshal(data []byte, v interface{}) error { // 反序列化
	iter := cfg.BorrowIterator(data)
	defer cfg.ReturnIterator(iter)
	iter.ReadVal(v)
	c := iter.nextToken()
	if c == 0 {
		if iter.Error == io.EOF {
			return nil
		}
		return iter.Error
	}
	iter.ReportError("Unmarshal", "there are bytes left after unmarshal")
	return iter.Error
}

func (cfg *frozenConfig) NewEncoder(writer io.Writer) *Encoder { //新建编码器
	stream := NewStream(cfg, writer, 512)
	return &Encoder{stream}
}

func (cfg *frozenConfig) NewDecoder(reader io.Reader) *Decoder { // 新建解码器
	iter := Parse(cfg, reader, 512)
	return &Decoder{iter}
}

// 验证数据
func (cfg *frozenConfig) Valid(data []byte) bool {
	iter := cfg.BorrowIterator(data)
	defer cfg.ReturnIterator(iter)
	iter.Skip()
	return iter.Error == nil
}
