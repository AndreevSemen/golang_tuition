package main

import (
	"encoding/json"
	"fmt"
	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)


//easyjson:json
type User struct {
	n int
	Name string       `json:"name"`
	EMail string      `json:"email"`
	Browsers []string `json:"browsers"`
}

func (u User) String() string {
	return fmt.Sprintf("[%d] %s <%s>\n", u.n, u.Name, u.EMail)
}


var filePool = sync.Pool{
	New: func() interface{} {
		file, err := os.Open(filePath)
		if err != nil {
			panic(err)
		}

		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}

		return string(fileContents)
	},
}

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	data := filePool.Get().(string)
	defer func() {
		filePool.Put(data)
	}()

	seenBrowsers := make(map[string]struct{})
	foundUsers := "found users:\n"

	for i := 0; ; i++ {
		index := strings.Index(data, "\n")
		if index == -1 {
			break
		}

		user := User{}
		err := user.UnmarshalJSON([]byte(data[:index]))
		if err != nil {
			panic(err)
		}

		data = data[index + 1:]


		isAndroid := false
		isMSIE := false

		for j := range user.Browsers {
			androidFound := strings.Contains(user.Browsers[j], "Android")
			if androidFound {
				isAndroid = true
			}
			MSIEFound := strings.Contains(user.Browsers[j], "MSIE")
			if MSIEFound {
				isMSIE = true
			}

			if androidFound || MSIEFound {
				if _, seen := seenBrowsers[user.Browsers[j]]; !seen {
					// log.Printf("SLOW New browser: %s, first seen: %s", browser, user["name"])
					seenBrowsers[user.Browsers[j]] = struct{}{}
				}
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		// log.Println("Android and MSIE user:", user["name"], user["email"])
		atIndex := strings.Index(user.EMail, "@")
		user.EMail = user.EMail[:atIndex] + " [at] " + user.EMail[atIndex + 1:]
		foundUsers += fmt.Sprintf("[%d] %s <%s>\n", i, user.Name, user.EMail)
	}

	fmt.Fprintln(out, foundUsers)
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)


func easyjson3486653aDecodeCourseraHw3BenchMake1(in *jlexer.Lexer, out *User) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "name":
			out.Name = string(in.String())
		case "email":
			out.EMail = string(in.String())
		case "browsers":
			if in.IsNull() {
				in.Skip()
				out.Browsers = nil
			} else {
				in.Delim('[')
				if out.Browsers == nil {
					if !in.IsDelim(']') {
						out.Browsers = make([]string, 0, 4)
					} else {
						out.Browsers = []string{}
					}
				} else {
					out.Browsers = (out.Browsers)[:0]
				}
				for !in.IsDelim(']') {
					var v4 string
					v4 = string(in.String())
					out.Browsers = append(out.Browsers, v4)
					in.WantComma()
				}
				in.Delim(']')
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson3486653aEncodeCourseraHw3BenchMake1(out *jwriter.Writer, in User) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"name\":"
		out.RawString(prefix[1:])
		out.String(string(in.Name))
	}
	{
		const prefix string = ",\"email\":"
		out.RawString(prefix)
		out.String(string(in.EMail))
	}
	{
		const prefix string = ",\"browsers\":"
		out.RawString(prefix)
		if in.Browsers == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v5, v6 := range in.Browsers {
				if v5 > 0 {
					out.RawByte(',')
				}
				out.String(string(v6))
			}
			out.RawByte(']')
		}
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v User) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson3486653aEncodeCourseraHw3BenchMake1(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v User) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson3486653aEncodeCourseraHw3BenchMake1(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *User) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson3486653aDecodeCourseraHw3BenchMake1(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *User) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson3486653aDecodeCourseraHw3BenchMake1(l, v)
}
