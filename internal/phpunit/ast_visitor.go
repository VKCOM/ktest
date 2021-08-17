package phpunit

import (
	"fmt"
	"strings"

	"github.com/z7zmey/php-parser/pkg/ast"
	"github.com/z7zmey/php-parser/pkg/visitor"
)

type astVisitor struct {
	visitor.Null
	out *testParsedInfo

	currentClass string
}

func (v *astVisitor) ExprMethodCall(n *ast.ExprMethodCall) {
	if v.currentClass != v.out.ClassName {
		return
	}
	object, ok := n.Var.(*ast.ExprVariable)
	if !ok {
		return
	}
	objectVar, ok := object.Name.(*ast.Identifier)
	if !ok || string(objectVar.Value) != "$this" {
		return
	}
	methodName, ok := n.Method.(*ast.Identifier)
	if !ok {
		return
	}
	switch string(methodName.Value) {
	case "assertTrue", "assertFalse", "assertSame", "assertNotSame", "assertEquals":
		lineMethodName := string(methodName.Value) + "WithLine"
		v.out.fixes = append(v.out.fixes, textEdit{
			StartPos:    methodName.GetPosition().StartPos,
			EndPos:      n.OpenParenthesisTkn.GetPosition().EndPos,
			Replacement: fmt.Sprintf("%s(__LINE__, ", lineMethodName),
		})
	}
}

func (v *astVisitor) StmtUse(n *ast.StmtUseList) {
	for _, u := range n.Uses {
		u := u.(*ast.StmtUse)
		if u.Type != nil {
			continue
		}
		name, ok := u.Use.(*ast.Name)
		if !ok {
			continue
		}
		if astNameToString(name) == `PHPUnit\Framework\TestCase` {
			pos := u.Use.GetPosition()
			v.out.fixes = append(v.out.fixes, textEdit{
				StartPos:    pos.StartPos,
				EndPos:      pos.EndPos,
				Replacement: `KPHPUnit\Framework\TestCase`,
			})
		}
	}
}

func (v *astVisitor) StmtClass(n *ast.StmtClass) {
	ident, ok := n.Name.(*ast.Identifier)
	if !ok {
		return
	}
	className := string(ident.Value)
	if !strings.HasSuffix(className, "Test") {
		return
	}
	v.out.ClassName = className
	v.currentClass = className
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
	if !strings.HasPrefix(methodName, "test") {
		return
	}
	v.out.TestMethods = append(v.out.TestMethods, methodName)
}
