//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
)

// Field represents a single field in a packet struct
type Field struct {
	Name      string // The Struct field name (e.g., "ProtocolVersion")
	FieldType string // The high-level type (e.g., "VarInt", "PrefixedArray", "Optional")
	WriteFn   string
	ReadFn    string
}

// GeneratedStruct represents a struct found in the source code marked for generation
type GeneratedStruct struct {
	Name              string
	Fields            []Field
	GenRead, GenWrite bool

	RegServerbound, RegClientbound bool
	PacketID                       string
}

type File struct {
	Name           string
	RegistryPrefix string
	Structs        []GeneratedStruct
}

// Helper methods for the template to check if we need to generate specific registries
func (f File) HasServerRegistry() bool {
	for _, s := range f.Structs {
		if s.RegServerbound {
			return true
		}
	}
	return false
}

func (f File) HasClientRegistry() bool {
	for _, s := range f.Structs {
		if s.RegClientbound {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) < 2 {
		// Default to current directory if no arg provided, or panic as per original requirement
		fmt.Println("Usage: go run gen_packet_codec.go -- path/to/dir")
		os.Exit(1)
	}

	targetDir := os.Args[len(os.Args)-1] // Take the last argument as the directory
	fset := token.NewFileSet()
	var parsedFiles []File
	var pkgName string

	filePaths, _ := filepath.Glob(filepath.Join(targetDir, "*.go"))

	for _, filePath := range filePaths {
		// Skip generated files to avoid double parsing
		if strings.HasPrefix(filepath.Base(filePath), "zz_generated") {
			continue
		}

		node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}

		if pkgName == "" {
			pkgName = node.Name.Name
		}

		var fileStructs []GeneratedStruct

		// Determine Registry Prefix (e.g., status.go -> Status)
		baseName := filepath.Base(filePath)
		namePart := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		registryPrefix := ""
		if len(namePart) > 0 {
			// Simple capitalization: status -> Status, login -> Login
			registryPrefix = strings.ToUpper(namePart[:1]) + namePart[1:]
		}

		// Pre-scan for ID() methods to map StructName -> ID
		structIDs := make(map[string]string)
		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			// Look for func (Receiver) ID() int32 { return X }
			if !ok || fn.Name.Name != "ID" || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}

			// Get Receiver Type Name
			var recvName string
			recvType := fn.Recv.List[0].Type
			if star, ok := recvType.(*ast.StarExpr); ok {
				if ident, ok := star.X.(*ast.Ident); ok {
					recvName = ident.Name
				}
			} else if ident, ok := recvType.(*ast.Ident); ok {
				recvName = ident.Name
			}

			if recvName == "" {
				continue
			}

			// Extract Return Value
			if fn.Body != nil {
				for _, stmt := range fn.Body.List {
					if ret, ok := stmt.(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
						if lit, ok := ret.Results[0].(*ast.BasicLit); ok {
							structIDs[recvName] = lit.Value
						}
					}
				}
			}
		}

		// Walk through top-level declarations
		for _, decl := range node.Decls {
			gen, ok := decl.(*ast.GenDecl)

			// filter for only type declarations with comments
			if !ok || gen.Tok != token.TYPE || gen.Doc == nil {
				continue
			}

			// Check for @gen marker and parse options
			var isGen bool
			var genRead, genWrite, regServer, regClient bool

			// Iterate through comments to find @gen line
			for _, comment := range gen.Doc.List {
				text := comment.Text
				// Check for options like @gen:r,w
				if strings.Contains(text, "@gen:") {
					isGen = true
					// Parse options
					parts := strings.Split(text, "@gen:")
					if len(parts) > 1 {
						opts := strings.Split(strings.TrimSpace(parts[1]), ",")
						for _, opt := range opts {
							opt = strings.TrimSpace(opt)
							if opt == "r" {
								genRead = true
							} else if opt == "w" {
								genWrite = true
							} else if opt == "regserver" {
								regServer = true
							} else if opt == "regclient" {
								regClient = true
							}
						}
					}
					break
				}
			}

			// filter for types with @gen in doc comment
			if !isGen {
				continue
			}

			for _, spec := range gen.Specs {
				tspec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				// type assertion for struct type
				structType, ok := tspec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				var fields []Field
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {

						// Get the raw tag string
						rawTag := ""
						if field.Tag != nil {
							// The Value is a BasicLit (string literal), strip the quotes
							rawTag = field.Tag.Value
							if len(rawTag) > 1 && rawTag[0] == '`' && rawTag[len(rawTag)-1] == '`' {
								rawTag = rawTag[1 : len(rawTag)-1] // Remove backticks
							}
						}

						// Use reflect.StructTag to parse the raw string
						parsedTag := reflect.StructTag(rawTag)
						fieldType := parsedTag.Get("field")
						writeFn := ""
						readFn := ""

						innerType := parsedTag.Get("inner")
						if len(innerType) > 0 {
							writeFn = "Write" + innerType
							readFn = "Read" + innerType
						} else {
							writeFn = parsedTag.Get("write")
							readFn = parsedTag.Get("read")
						}

						if fieldType == "" {
							continue // Skip fields without the "field" tag
						}

						f := Field{
							Name:      name.Name,
							FieldType: fieldType,
							WriteFn:   writeFn,
							ReadFn:    readFn,
						}

						fields = append(fields, f)
					}
				}

				fileStructs = append(fileStructs, GeneratedStruct{
					Name:           tspec.Name.Name,
					Fields:         fields,
					GenRead:        genRead,
					GenWrite:       genWrite,
					RegServerbound: regServer,
					RegClientbound: regClient,
					PacketID:       structIDs[tspec.Name.Name],
				})
			}

		}
		if len(fileStructs) > 0 {
			parsedFiles = append(parsedFiles, File{
				Name:           filepath.Base(filePath),
				RegistryPrefix: registryPrefix,
				Structs:        fileStructs,
			})
		}
	}

	// Output next to the source files
	outFile := filepath.Join(targetDir, "zz_generated_codec.go")
	out, err := os.Create(outFile)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	// Use a template for cleaner code generation logic
	const tmpl = `// Code generated by gen_packet_codec.go; DO NOT EDIT.
package {{.PkgName}}

import (
	"io"
)
{{range .Files}}
// Source: {{.Name}}
{{- if .HasServerRegistry}}
var {{.RegistryPrefix}}ServerboundRegistry = map[int32]func() Packet{
{{- range .Structs}}
	{{- if .RegServerbound}}
	{{.PacketID}}: func() Packet { return &{{.Name}}{} },
	{{- end}}
{{- end}}
}
{{- end}}
{{- if .HasClientRegistry}}
var {{.RegistryPrefix}}ClientboundRegistry = map[int32]func() Packet{
{{- range .Structs}}
	{{- if .RegClientbound}}
	{{.PacketID}}: func() Packet { return &{{.Name}}{} },
	{{- end}}
{{- end}}
}
{{- end}}
{{range .Structs}}
{{- if .GenWrite}}
func (p {{.Name}}) Encode(w io.Writer) (err error) {
	if err = WriteVarInt(w, p.ID()); err != nil { return }
{{- range .Fields}}
	{{- if .WriteFn}}
	if err = Write{{.FieldType}}(w, p.{{.Name}}, {{.WriteFn}}); err != nil { return }
	{{- else}}
	if err = Write{{.FieldType}}(w, p.{{.Name}}); err != nil { return }
	{{- end}}
{{- end}}
	return
}
{{- end}}
{{if .GenRead}}
func (p *{{.Name}}) Decode(r *FrameReader) (err error) {
{{- range .Fields}}
	{{- if .ReadFn}}
	if p.{{.Name}}, err = Read{{.FieldType}}(r, {{.ReadFn}}); err != nil { return }
	{{- else}}
	if p.{{.Name}}, err = Read{{.FieldType}}(r); err != nil { return }
	{{- end}}
{{- end}}
	return nil
}
{{- end}}
{{end}}
{{- end}}
`

	t := template.Must(template.New("code").Parse(tmpl))
	data := struct {
		PkgName string
		Files   []File
	}{
		PkgName: pkgName,
		Files:   parsedFiles,
	}

	if err := t.Execute(out, data); err != nil {
		panic(err)
	}

	fmt.Printf("Generated %s for package %s\n", outFile, pkgName)
}
