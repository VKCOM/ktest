package bench

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/z7zmey/php-parser/pkg/ast"
	"github.com/z7zmey/php-parser/pkg/visitor"
)

type astVisitor struct {
	visitor.Null
	out *benchParsedInfo

	issues []error

	currentFileName  string
	currentNamespace string
	currentClass     string
}

func (v *astVisitor) StmtNamespace(n *ast.StmtNamespace) {
	ident, ok := n.Name.(*ast.Name)
	if !ok {
		return
	}

	namespace := strings.Builder{}
	for _, part := range ident.Parts {
		partIdent, ok := part.(*ast.NamePart)
		if !ok {
			continue
		}

		namespace.Write(partIdent.Value)
		namespace.WriteByte('\\')
	}

	v.currentNamespace = namespace.String()
}

func (v *astVisitor) StmtClass(n *ast.StmtClass) {
	ident, ok := n.Name.(*ast.Identifier)
	if !ok {
		return
	}
	className := string(ident.Value)
	fqn := "\\" + v.currentNamespace + className

	if strings.HasPrefix(className, "Benchmark") {
		v.out.ClassName = className
		v.out.ClassFQN = fqn
		v.currentClass = fqn

		v.checkThatClassNameMatchesFilename(className)
		return
	}

	if strings.HasSuffix(className, "Benchmark") {
		withoutBenchmarkSuffix := strings.TrimSuffix(className, "Benchmark")

		v.issues = append(v.issues,
			fmt.Errorf(
				`perhaps you meant 'Benchmark%s', class name should be prefixed with 'Benchmark' and not suffixed`,
				withoutBenchmarkSuffix,
			),
		)
	}
}

func (v *astVisitor) checkThatClassNameMatchesFilename(className string) {
	fileName := filepath.Base(v.currentFileName)
	if fileName != className+".php" {
		v.issues = append(v.issues,
			fmt.Errorf("filename '%s' does not match the class name '%s' of the benchmark.\n"+
				"KPHP will not be able to find the class.\n\n"+
				"To fix, name the file '%s'", fileName, className, className+".php"),
		)
	}
}

func (v *astVisitor) StmtClassMethod(n *ast.StmtClassMethod) {
	if v.currentClass != v.out.ClassFQN {
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
		Key:  strings.TrimPrefix(strings.TrimPrefix(methodName, "benchmark"), "_"),
	})
}
