package jsoniter

import (
	"bytes"
	"io"
)

// RawMessage to make replace json with jsoniter
type RawMessage []byte

// Unmarshal adapts to json/encoding Unmarshal API
//
// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
// Refer to https://godoc.org/encoding/json#Unmarshal for more information
func Unmarshal(data []byte, v interface{}) error {   // 反序列化  结果存放在v中
	return ConfigDefault.Unmarshal(data, v)
}

// UnmarshalFromString convenient method to read from string instead of []byte
func UnmarshalFromString(str string, v interface{}) error {  // 从字符串进行反序列化  结果存放在v中
	return ConfigDefault.UnmarshalFromString(str, v)
}

/* 假设data := ([]byte)(`{"ID":1,"Name":"Reds","Colors":{"c":"Crimson","r":"Red","rb":"Ruby","m":"Maroon","tests":["tests_1","tests_2","tests_3","tests_4"]}}`)
 * 当对应的节点是存放在{} 直接 jsoniter.Get(data, "Colors").ToString() => 输出{"c":"Crimson","r":"Red","rb":"Ruby","m":"Maroon","tests":["tests_1","tests_2","tests_3","tests_4"]}
 * 当对应的节点是存放在[] 直接  jsoniter.Get(val, "Colors","tests",0).ToString() => 输出 tests_1 注多个元素存放在[] 则可指定对应的索引获取对应的数据
 */
// Get quick method to get value from deeply nested JSON structure
func Get(data []byte, path ...interface{}) Any {  // 从嵌套的json结构中获取对应的value 当Get(searches, "Colors",0) 则代表获取json结构中Colors节点第一个内容
	return ConfigDefault.Get(data, path...)
}

// 输出结果格式： {"ID":1,"Name":"Reds","Colors":["Crimson","Red","Ruby","Maroon"]}
// Marshal adapts to json/encoding Marshal API
//
// Marshal returns the JSON encoding of v, adapts to json/encoding Marshal API
// Refer to https://godoc.org/encoding/json#Marshal for more information
func Marshal(v interface{}) ([]byte, error) {  // 序列化  结果输出到[]byte
	return ConfigDefault.Marshal(v)
}

/* 输出结果格式：
	{
	 "ID": 1,
	 "Name": "Reds",
	 "Colors": [
	  "Crimson",
	  "Red",
	  "Ruby",
	  "Maroon"
	 ]
	}
*/
// MarshalIndent same as json.MarshalIndent. Prefix is not supported.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return ConfigDefault.MarshalIndent(v, prefix, indent)
}

// MarshalToString convenient method to write as string instead of []byte
func MarshalToString(v interface{}) (string, error) {
	return ConfigDefault.MarshalToString(v)
}

// ======================== 解码：Decoder=============================
// NewDecoder adapts to json/stream NewDecoder API.
//
// NewDecoder returns a new decoder that reads from r.
//
// Instead of a json/encoding Decoder, an Decoder is returned
// Refer to https://godoc.org/encoding/json#NewDecoder for more information
func NewDecoder(reader io.Reader) *Decoder {
	return ConfigDefault.NewDecoder(reader)
}

// Decoder reads and decodes JSON values from an input stream.
// Decoder provides identical APIs with json/stream Decoder (Token() and UseNumber() are in progress)
type Decoder struct {
	iter *Iterator
}

// Decode decode JSON into interface{}
func (adapter *Decoder) Decode(obj interface{}) error {
	if adapter.iter.head == adapter.iter.tail && adapter.iter.reader != nil {
		if !adapter.iter.loadMore() {
			return io.EOF
		}
	}
	adapter.iter.ReadVal(obj)
	err := adapter.iter.Error
	if err == io.EOF {
		return nil
	}
	return adapter.iter.Error
}

// More is there more?
func (adapter *Decoder) More() bool {
	iter := adapter.iter
	if iter.Error != nil {
		return false
	}
	c := iter.nextToken()
	if c == 0 {
		return false
	}
	iter.unreadByte()
	return c != ']' && c != '}'
}

// Buffered remaining buffer
func (adapter *Decoder) Buffered() io.Reader {
	remaining := adapter.iter.buf[adapter.iter.head:adapter.iter.tail]
	return bytes.NewReader(remaining)
}

// UseNumber causes the Decoder to unmarshal a number into an interface{} as a
// Number instead of as a float64.
func (adapter *Decoder) UseNumber() {
	cfg := adapter.iter.cfg.configBeforeFrozen
	cfg.UseNumber = true
	adapter.iter.cfg = cfg.frozeWithCacheReuse(adapter.iter.cfg.extraExtensions)
}

// DisallowUnknownFields causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func (adapter *Decoder) DisallowUnknownFields() {
	cfg := adapter.iter.cfg.configBeforeFrozen
	cfg.DisallowUnknownFields = true
	adapter.iter.cfg = cfg.frozeWithCacheReuse(adapter.iter.cfg.extraExtensions)
}

// ===================== 编码 Encoder ===========================
// NewEncoder same as json.NewEncoder
func NewEncoder(writer io.Writer) *Encoder {
	return ConfigDefault.NewEncoder(writer)
}

// Encoder same as json.Encoder
type Encoder struct {
	stream *Stream
}

// Encode encode interface{} as JSON to io.Writer
func (adapter *Encoder) Encode(val interface{}) error {
	adapter.stream.WriteVal(val)
	adapter.stream.WriteRaw("\n")
	adapter.stream.Flush()
	return adapter.stream.Error
}

// SetIndent set the indention. Prefix is not supported
func (adapter *Encoder) SetIndent(prefix, indent string) {
	config := adapter.stream.cfg.configBeforeFrozen
	config.IndentionStep = len(indent)
	adapter.stream.cfg = config.frozeWithCacheReuse(adapter.stream.cfg.extraExtensions)
}

// SetEscapeHTML escape html by default, set to false to disable
func (adapter *Encoder) SetEscapeHTML(escapeHTML bool) {
	config := adapter.stream.cfg.configBeforeFrozen
	config.EscapeHTML = escapeHTML
	adapter.stream.cfg = config.frozeWithCacheReuse(adapter.stream.cfg.extraExtensions)
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	return ConfigDefault.Valid(data)
}
