/*
 *  iterator-api：用于处理超大的输入
 *  bind-api：日常最经常使用的对象绑定
 *  any-api：lazy 解析大对象，具有 PHP Array 一般的使用体验
 */

package main

import (
	"fmt"
	"github.com/json-iterator/go"
	"os"
	"strings"
)

type ColorGroup struct {
	ID 		int
	Name    string
	Colors  []string
}

type Animal struct {
	Name	string
	Order 	string
}

func main() {
	// ================= 序列化 =====================
	group := ColorGroup{
		ID:		1,
		Name:   "Reds",
		Colors: []string{"Crimson", "Red", "Ruby", "Maroon"},
	}
	b, err := jsoniter.Marshal(group)
	bb, err :=  jsoniter.MarshalIndent(group, "", " ")
	if err != nil{
		fmt.Println("error: ", err)
	}
	os.Stdout.Write(b)
	fmt.Println()
	os.Stdout.Write(bb)
	fmt.Println()

	// ===================  Deconde 解码 =================
	jsoniter.NewDecoder(os.Stdin).Decode(&group)
	fmt.Println(group)

	encoder := jsoniter.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(true)
	encoder.Encode(bb)
	fmt.Println(string(bb))

	// =================== 反序列化 =======================
	var jsonBlob = []byte(`[
		{"Name": "Platypus", "Order": "Monotremata"},
		{"Name": "Quoll",    "Order": "Dasyuromorphia"}
	]`)
	var animals []Animal
	if err := jsoniter.Unmarshal(jsonBlob, &animals); err != nil{
		fmt.Println("error: ", err)
	}

	fmt.Printf("the unmarshal is  %+v", animals)

	// ======================= 流式 ========================
	fmt.Println()

	// 序列化
	stream := jsoniter.ConfigFastest.BorrowStream(nil)
	defer jsoniter.ConfigFastest.ReturnStream(stream)
	stream.WriteVal(group)
	if stream.Error != nil{
		fmt.Println("error: ", stream.Error)
	}
	os.Stdout.Write(stream.Buffer())

	fmt.Println()
	// 反序列化
	iter := jsoniter.ConfigFastest.BorrowIterator(jsonBlob)
	defer jsoniter.ConfigFastest.ReturnIterator(iter)
	iter.ReadVal(&animals)
	if iter.Error != nil{
		fmt.Println("error： ", iter.Error)
	}
	fmt.Printf("%+v", animals)

	fmt.Println()
	// ====================其他操作===================
	// get
	val := []byte(`{"ID":1,"Name":"Reds","Colors":{"c":"Crimson","r":"Red","rb":"Ruby","m":"Maroon","tests":["tests_1","tests_2","tests_3","tests_4"]}}`)
	fmt.Println(jsoniter.Get(val, "Colors").ToString())
	fmt.Println("the result is " , jsoniter.Get(val, "Colors","tests",0).ToString())
	// fmt.Println(jsoniter.Get(val, "colors", 0).ToString())

	fmt.Println()
	hello := MyKey("hello")
	output, _ := jsoniter.Marshal(map[*MyKey]string{&hello: "world"})
	fmt.Println(string(output))

	obj := map[*MyKey]string{}
	jsoniter.Unmarshal(output, &obj)
	for k, v := range obj{
		fmt.Println(*k," = ", v)
	}

}
// 自定义类型
// 序列化： 需要实现MarshellText
type MyKey string

func (m *MyKey) MarshalText() ([]byte, error){
	// return []byte(string(*m)) , nil  // 针对序列化的内容不做任何调整
	return []byte(strings.Replace(string(*m), "h","H",-1)), nil
}

func(m *MyKey) UnmarshalText(text []byte) error{
	*m = MyKey(text[:])  // 针对text不做处理
	return nil
}
