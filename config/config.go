package config

import (
	"GoRedis/lib/logger"
	"bufio"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type ServerProperties struct {
	Bind           string `cfg:"bind"`
	Port           int    `cfg:"port"`
	AppendOnly     bool   `cfg:"appendOnly"`
	AppendFilename string `cfg:"appendFilename"`
	MaxClients     int    `cfg:"maxclients "`
	RequirePass    string `cfg:"requirepass"`
	Databases      int    `cfg:"databases"`

	Peers []string `cfg:"peers"`
	Self  string   `cfg:"self"`
}

var Properties *ServerProperties

func init() {
	// 默认配置
	Properties = &ServerProperties{
		Bind:       "127.0.0.1",
		Port:       63791,
		AppendOnly: false,
	}
}

// 解析配置文件
func parse(src io.Reader) *ServerProperties {
	config := &ServerProperties{}

	// 读取配置文件
	// 储存每项配置对应的参数
	rawMap := make(map[string]string)
	scanner := bufio.NewScanner(src)

	// 逐行读取
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && line[0] == '#' {
			continue
		}
		pivot := strings.IndexAny(line, " ")
		// 将配置信息存入map
		if pivot > 0 && pivot < len(line)-1 {
			// 取key
			key := line[0:pivot]
			// 取value
			// 删除value句尾的""字符
			value := strings.Trim(line[pivot+1:], "")
			rawMap[strings.ToLower(key)] = value
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Fatal(err)
	}

	// 将配置信息映射到config结构体
	t := reflect.TypeOf(config)
	v := reflect.ValueOf(config)
	// 获取config结构体的成员数量
	n := t.Elem().NumField()
	for i := 0; i < n; i++ {
		field := t.Elem().Field(i)
		fieldVal := v.Elem().Field(i)
		key, ok := field.Tag.Lookup("cfg")
		if !ok {
			key = field.Name
		}
		value, ok := rawMap[strings.ToLower(key)]
		if ok {
			// 填充配置信息
			switch field.Type.Kind() {
			case reflect.String:
				fieldVal.SetString(value)
			case reflect.Int:
				intValue, err := strconv.ParseInt(value, 10, 64)
				if err == nil {
					fieldVal.SetInt(intValue)
				}
			case reflect.Bool:
				boolValue := "yes" == value
				fieldVal.SetBool(boolValue)
			case reflect.Slice:
				if field.Type.Elem().Kind() == reflect.String {
					slice := strings.Split(value, ",")
					fieldVal.Set(reflect.ValueOf(slice))
				}
			}
		}
	}
	return config
}

// SetupConfig 打开配置文件并解析
func SetupConfig(configFilename string) {
	file, err := os.Open(configFilename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	Properties = parse(file)
}
