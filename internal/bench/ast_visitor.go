package bench

import (
	"strings"

	"github.com/z7zmey/php-parser/pkg/ast"
	"github.com/z7zmey/php-parser/pkg/visitor"
)

type astVisitor struct {
	visitor.Null
	out *benchParsedInfo

	currentClass string
}

func (v *astVisitor) StmtClass(n *ast.StmtClass) {
	ident, ok := n.Name.(*ast.Identifier)
	if !ok {
		return
	}
	className := string(ident.Value)
	if strings.HasPrefix(className, "Benchmark") {
		v.out.ClassName = className
		v.currentClass = className
	}
}

func (v *astVisitor) StmtClassMethod(n *ast.StmtClassMethod) {
	if v.currentClass != v.out.ClassName {
		return
	}
	ident, ok := n.Name.(*ast.Identifier)
	if !ok {
		return
	}
	methodName := string(ident.Value)
	if !strings.HasPrefix(methodName, "benchmark") {
		return
	}
	v.out.BenchMethods = append(v.out.BenchMethods, benchMethod{
		Name: methodName,
		Key:  strings.TrimPrefix(methodName, "benchmark"),
	})
}
