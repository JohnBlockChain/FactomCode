package main 

import (
	"flag"
	"net/http"
	"net/url"
	ncdata "NotaryChain/data"
	"encoding/json"
	"encoding/xml"
	"strconv"
	"fmt"
	"strings"
	"reflect"
	"time"
	"errors"
)

var portNumber *int = flag.Int("p", 8083, "Set the port to listen on")

var blocks []*ncdata.Block

func load() {
	source := `[
		{
			"blockID": 0,
			"previousHash": null,
			"entries": [],
			"salt": {
				"bytes": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
			}
		},
		{
			"blockID": 1,
			"previousHash": {
				"bytes": "LDTOHfI7g4xavyp/ZDfMo9MGftUJ/yXxHfaxG1grUes="
			},
			"entries": [
				{
					"entryType": 0,
					"structuredData": "EBESEw==",
					"signatures": [],
					"timeSamp": 0
				}
			],
			"salt": {
				"bytes": "HJ7OyQ4o0kYWUEGGNYeKXJHkn0dYbs918rDLuU6JcRI="
			}
		}
	]`
	
	if err := json.Unmarshal([]byte(source), &blocks); err != nil {
		panic(err)
	}
	
	for i := 0; i < len(blocks); i = i + 1 {
		if uint64(i) != blocks[i].BlockID {
			panic(errors.New("BlockID does not index"))
		}
	}
}

func main() {
	load()
	
	http.HandleFunc("/", serveRESTfulHTTP)
	http.ListenAndServe(":" + strconv.Itoa(*portNumber), nil)
}

func serveRESTfulHTTP(w http.ResponseWriter, r *http.Request) {
	var resource interface{}
	var data []byte
	var err *restError
	
	path, method, accept, form, err := parse(r)
	
	defer func() {
		switch accept {
		case "text":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			
		case "json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			
		case "xml":
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			
		case "html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
	
		if err != nil {
			var r *restError
			
			data, r = marshal(err, accept)
			if r != nil {
				err = r
			}
			w.WriteHeader(err.HTTPCode)
		}
		
		w.Write(data)
		w.Write([]byte("\n\n"))
	}()
	
	switch method {
	case "GET":
		resource, err = find(path)
		
	case "POST":
		if len(path) != 1 {
			err = createError(errorBadMethod, `POST can only be used in the root context: /v1`)
			return
		}
		
		resource, err = post("/" + strings.Join(path, "/"), blocks[len(blocks) - 1], form)
		
	default:
		err = createError(errorBadMethod, fmt.Sprintf(`The HTTP %s method is not supported`, method))
		return
	}
	
	data, err = marshal(resource, accept)
}

var blockPtrType = reflect.TypeOf((*ncdata.Block)(nil)).Elem()

func post(context string, resource interface{}, form url.Values) (interface{}, *restError) {
	resourceType := reflect.ValueOf(resource).Elem().Type()
	if blockPtrType != resourceType {
		return nil, createError(errorBadMethod, fmt.Sprintf(`POST is not supported on type %s`, resourceType.String()))
	}
	
	newEntry := new(ncdata.PlainEntry)
	format, data := form.Get("format"), form.Get("data")
	
	switch format {
	case "", "json":
		err := json.Unmarshal([]byte(data), newEntry)
		if err != nil {
			return nil, createError(errorJSONUnmarshal, err.Error())
		}
		
	case "xml":
		err := xml.Unmarshal([]byte(data), newEntry)
		if err != nil {
			return nil, createError(errorXMLUnmarshal, err.Error())
		}
	
	default:
		return nil, createError(errorUnsupportedUnmarshal, fmt.Sprintf(`The format "%s" is not supported`, format))
	}
	
	if newEntry == nil {
		return nil, createError(errorInternal, `Entity to be POSTed is nil`)
	}
	
	newEntry.TimeStamp = time.Now().Unix()
	
	err := resource.(*ncdata.Block).AddEntry(newEntry)
	if err != nil {
		return nil, createError(errorInternal, fmt.Sprintf(`Error while adding Entity to Block: %s`, err.Error()))
	}
	
	return newEntry, nil
}

func marshal(resource interface{}, accept string) (data []byte, r *restError) {
	var err error
	
	switch accept {
	case "text":
		data, err = json.MarshalIndent(resource, "", "  ")
		if err != nil {
			r = createError(errorJSONMarshal, err.Error())
			data, err = json.MarshalIndent(r, "", "  ")
			if err != nil {
				panic(err)
			}
		}
		return
		
	case "json":
		data, err = json.Marshal(resource)
		if err != nil {
			r = createError(errorJSONMarshal, err.Error())
			data, err = json.Marshal(r)
			if err != nil {
				panic(err)
			}
		}
		return
		
	case "xml":
		data, err = xml.Marshal(resource)
		if err != nil {
			r = createError(errorXMLMarshal, err.Error())
			data, err = xml.Marshal(r)
			if err != nil {
				panic(err)
			}
		}
		return
		
	case "html":
		data, r = marshal(resource, "json")
		if r != nil {
			return nil, r
		}
		data = []byte(fmt.Sprintf(`<script>
			function tree(data) {
			    if (typeof(data) == 'object') {
			        document.write('<ul>');
			        for (var i in data) {
			            document.write('<li>' + i);
			            tree(data[i]);
			        }
			        document.write('</ul>');
			    } else {
			        document.write(' => ' + data);
			    }
			}</script><body onload='tree(%s)'></body>`, data))
		return
	}
	
	r  = createError(errorUnsupportedMarshal, fmt.Sprintf(`"%s" is an unsupported marshalling format`, accept))
	data, err = json.Marshal(r)
	if err != nil {
		panic(err)
	}
	return
}







