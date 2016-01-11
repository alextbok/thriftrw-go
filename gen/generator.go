// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package gen

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"text/template"
)

// Generator tracks code generation state as we generate the output.
type Generator struct {
	importer

	decls []ast.Decl

	// TODO use something to group related decls together

	// TODO(abg) We will keep track of needed map/list/set types and their
	// to/from value implementations here
}

// NewGenerator sets up a new generator for Go code.
func NewGenerator() *Generator {
	return &Generator{importer: newImporter()}
}

func (g *Generator) renderTemplate(s string, data interface{}) ([]byte, error) {
	templateFuncs := template.FuncMap{
		"goCase":  goCase,
		"import":  g.Import,
		"defName": typeDeclName,

		"typeReference": typeReference,
		"Required":      func() fieldRequired { return Required },
		"Optional":      func() fieldRequired { return Optional },
		"required": func(b bool) fieldRequired {
			if b {
				return Required
			}
			return Optional
		},
	}
	// TODO(abg): Add functions like "newVar" so that templates don't have to.

	tmpl, err := template.New("thriftrw").Funcs(templateFuncs).Parse(s)
	if err != nil {
		return nil, err
	}

	buff := bytes.NewBufferString("package thriftrw\n\n")
	if err := tmpl.Execute(buff, data); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// DeclareFromTemplate renders a template (in the text/template format) that
// generates Go code and includes all declarations from the template in the code
// generated by the generator.
//
// An error is returned if anything went wrong while generating the template.
//
// For example,
//
// 	g.DeclareFromTemplate(
// 		'type {{ .Name }} int32',
// 		struct{Name string}{Name: "myType"}
// 	)
//
// Will generate,
//
// 	type myType int32
//
// The following functions are available to templates:
//
// goCase(str): Accepts a string and returns it in CamelCase form and the first
// character upper-cased. The string may be ALLCAPS, snake_case, or already
// camelCase.
//
// import(str): Accepts a string and returns the name that should be used in the
// template to refer to that imported module. This helps avoid naming conflicts
// with imports.
//
// 	{{ $fmt := import "fmt" }}
// 	{{ $fmt }}.Println("hello world")
//
// defName(TypeSpec): Takes a TypeSpec representing a **user declared type** and
// returns the name that should be used in the Go code to define that type.
//
// typeReference(TypeSpec, fieldRequired): Takes any TypeSpec and a a value
// indicating whether this reference expects the type to always be present (use
// the "required" function on a boolean, or the "Required" and "Optional"
// functions inside the template to get the corresponding fieldRequired value).
// Returns a string representing a reference to that type, wrapped in a pointer
// if the value was optional.
//
// 	{{ typeReference $someType Required }}
func (g *Generator) DeclareFromTemplate(s string, data interface{}) error {
	bs, err := g.renderTemplate(s, data)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(token.NewFileSet(), "thriftrw.go", bs, 0)
	if err != nil {
		return err
	}

	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || d.Tok != token.IMPORT {
			g.appendDecl(decl)
			continue
		}

		imports := d.Specs
		for _, imp := range imports {
			// TODO: May have to rewrite imports in the other decls of the
			// parsed AST if there are collisions here.
			//
			// Although we should just use the {{ import }} function instead.
			g.addImportSpec(imp.(*ast.ImportSpec))
		}
	}

	return nil
}

// TODO mutliple modules

func (g *Generator) Write(w io.Writer, fs *token.FileSet) error {
	// TODO newlines between decls
	// TODO constants first, types next, and functions after that
	// TODO sorting

	decls := make([]ast.Decl, 0, 1+len(g.decls))
	importDecl := g.importDecl()
	if importDecl != nil {
		decls = append(decls, importDecl)
	}
	decls = append(decls, g.decls...)

	file := &ast.File{
		Decls: decls,
		Name:  ast.NewIdent("todo"), // TODO
	}
	return format.Node(w, fs, file)
}

// appendDecl appends a new declaration to the generator.
func (g *Generator) appendDecl(decl ast.Decl) {
	g.decls = append(g.decls, decl)
}
