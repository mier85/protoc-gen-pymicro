package main

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	plugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/iancoleman/strcase"
)

var classTemplate = `import {{.Path}}
from Micro import Client


class {{.Name}}:
    client = None
    service_name = ""

    def __init__(self, service_name: str, micro_client: Client):
        self.client = micro_client
        self.service_name = service_name
{{range .Methods}}
    def {{.Name}}(self, request: {{.Path}}.{{.Request}}) \
            -> {{.Path}}.{{.Response}}:
        response = {{.Path}}.{{.Response}}()
        error = self.client.call(self.service_name, "{{.ServiceMethodName}}", request, response)
        if error != "":
            raise ValueError(error)
        return response
{{end}}
`

type Method struct {
	Name string
	Path string
	Request string
	Response string
	ServiceMethodName string
}

type Class struct {
	Path string
	Name string
	Methods []Method
}

var codeTemplate = template.Must(template.New("pymicro").Parse(classTemplate))

func generateMypyStubs(
	req *plugin.CodeGeneratorRequest) *plugin.CodeGeneratorResponse {

	resp := &plugin.CodeGeneratorResponse{}

	files := []*plugin.CodeGeneratorResponse_File{}
	for _, file := range req.ProtoFile {
		pathName := strings.TrimSuffix(file.GetName(), ".proto") + "_pb2"
		fileName := strings.TrimSuffix(file.GetName(), ".proto") + "_micro_pb2.py"
		buf := &bytes.Buffer{}
		for _, service := range file.Service {
			c  := &Class{Name:fmt.Sprintf("%sClient", *service.Name), Path: pathName}
			methods := make([]Method, 0, len(service.Method))

			for _, method := range service.Method {
				output := strings.Split(*method.OutputType, ".")
				input := strings.Split(*method.InputType, ".")
				name := *service.Name+"."+*method.Name
				methods = append(methods, Method{
					Path: pathName,
					Name:strcase.ToSnake(*method.Name),
					ServiceMethodName: name,
					Request: input[len(input) - 1],
					Response: output[len(output) - 1],
				})
			}
			c.Methods = methods
			if err := codeTemplate.Execute(buf, c); nil != err {
				panic(err)
			}
		}
		content := buf.String()
		if content == "" {
			continue
		}
		files = append(files, &plugin.CodeGeneratorResponse_File{
			Name: &fileName,
			Content: &content,
		})
	}
	resp.File = files
	return resp
}

func main() {
	req := command.Read()
	command.Write(generateMypyStubs(req))
}
