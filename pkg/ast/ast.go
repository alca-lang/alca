// Package ast defines the abstract syntax tree (AST) node types and interfaces
// for the Alca programming language.
package ast

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/token"
)

// Node is the foundational interface that all AST nodes in Alca must implement.
type Node interface {
	// TokenLiteral returns the literal text of the primary token of this node.
	TokenLiteral() string
	// String returns a human-readable debug representation of this node.
	String() string
}

// Statement represents an executable statement node in the Alca AST.
type Statement interface {
	Node
	statementNode()
}

// Expression represents an evaluable expression node in the Alca AST.
type Expression interface {
	Node
	expressionNode()
}

// -----------------------------------------------------------------------------
// Statement Nodes
// -----------------------------------------------------------------------------

// Program is the root node of every Alca abstract syntax tree.
type Program struct {
	Token      token.Token // Token of the initial statement or EOF
	Statements []Statement // Sequence of statements in the program
}

func (p *Program) statementNode() {}

// TokenLiteral returns the literal value of the root token or the first statement.
func (p *Program) TokenLiteral() string {
	if p.Token.Literal != "" {
		return p.Token.Literal
	}
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

// String returns the concatenated string representation of all program statements.
func (p *Program) String() string {
	var out strings.Builder
	for _, s := range p.Statements {
		if s != nil {
			out.WriteString(s.String())
			out.WriteString("\n")
		}
	}
	return out.String()
}

// VarDeclaration represents a variable declaration and optional assignment.
type VarDeclaration struct {
	Token          token.Token     // Name token or assignment token
	Decorators     []*Decorator    // Annotations applied to the variable
	Name           *Identifier     // Name of the variable
	TypeAnnotation *TypeAnnotation // Explicit type annotation, or nil for inferred type
	Value          Expression      // Initializing expression, or nil
}

func (vd *VarDeclaration) statementNode() {}

// TokenLiteral returns the literal representation of the declaration token.
func (vd *VarDeclaration) TokenLiteral() string { return vd.Token.Literal }

// String returns the string representation of the variable declaration.
func (vd *VarDeclaration) String() string {
	var out strings.Builder
	for _, d := range vd.Decorators {
		if d != nil {
			out.WriteString(d.String())
			out.WriteString("\n")
		}
	}
	if vd.Name != nil {
		out.WriteString(vd.Name.String())
	}
	if vd.TypeAnnotation != nil {
		out.WriteString(": ")
		out.WriteString(vd.TypeAnnotation.String())
	}
	if vd.Value != nil {
		out.WriteString(" = ")
		out.WriteString(vd.Value.String())
	}
	return out.String()
}

// FuncDeclaration represents a function or shell-integrated function declaration.
type FuncDeclaration struct {
	Token      token.Token     // token.FUNC or token.SHFUNC
	Decorators []*Decorator    // Annotations applied to the function
	IsPublic   bool            // True if exported via 'pub'
	IsShell    bool            // True if declared as 'shfunc'
	Name       string          // Name of the function
	Params     []*FuncParam    // Function parameters
	ReturnType *TypeAnnotation // Declared return type, or nil if omitted
	Body       *BlockStatement // Function body
}

func (fd *FuncDeclaration) statementNode() {}

// TokenLiteral returns the literal representation of the function token.
func (fd *FuncDeclaration) TokenLiteral() string { return fd.Token.Literal }

// String returns the string representation of the function declaration.
func (fd *FuncDeclaration) String() string {
	var out strings.Builder
	for _, d := range fd.Decorators {
		if d != nil {
			out.WriteString(d.String())
			out.WriteString("\n")
		}
	}
	if fd.IsPublic {
		out.WriteString("pub ")
	}
	if fd.IsShell {
		out.WriteString("shfunc ")
	} else {
		out.WriteString("func ")
	}
	out.WriteString(fd.Name)
	out.WriteString("(")
	params := make([]string, 0, len(fd.Params))
	for _, p := range fd.Params {
		if p != nil {
			params = append(params, p.String())
		}
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if fd.ReturnType != nil {
		out.WriteString(" -> ")
		out.WriteString(fd.ReturnType.String())
	}
	if fd.Body != nil {
		out.WriteString(" ")
		out.WriteString(fd.Body.String())
	}
	return out.String()
}

// FuncParam represents an individual parameter in a function or lambda signature.
type FuncParam struct {
	Token          token.Token     // Identifier token of parameter
	Name           string          // Parameter name
	TypeAnnotation *TypeAnnotation // Parameter type, or nil
	DefaultValue   Expression      // Default expression if omitted, or nil
}

func (fp *FuncParam) statementNode() {}

// TokenLiteral returns the literal parameter name token.
func (fp *FuncParam) TokenLiteral() string { return fp.Token.Literal }

// String returns the string representation of the parameter.
func (fp *FuncParam) String() string {
	var out strings.Builder
	out.WriteString(fp.Name)
	if fp.TypeAnnotation != nil {
		out.WriteString(": ")
		out.WriteString(fp.TypeAnnotation.String())
	}
	if fp.DefaultValue != nil {
		out.WriteString(" = ")
		out.WriteString(fp.DefaultValue.String())
	}
	return out.String()
}

// StructDeclaration represents a user-defined struct type declaration.
type StructDeclaration struct {
	Token      token.Token    // token.STRUCT
	Decorators []*Decorator   // Annotations applied to the struct
	IsPublic   bool           // True if exported via 'pub'
	Name       string         // Struct name
	Fields     []*StructField // Struct member fields
}

func (sd *StructDeclaration) statementNode() {}

// TokenLiteral returns the literal representation of the struct token.
func (sd *StructDeclaration) TokenLiteral() string { return sd.Token.Literal }

// String returns the string representation of the struct declaration.
func (sd *StructDeclaration) String() string {
	var out strings.Builder
	for _, d := range sd.Decorators {
		if d != nil {
			out.WriteString(d.String())
			out.WriteString("\n")
		}
	}
	if sd.IsPublic {
		out.WriteString("pub ")
	}
	out.WriteString("struct ")
	out.WriteString(sd.Name)
	out.WriteString(" {\n")
	for _, f := range sd.Fields {
		if f != nil {
			out.WriteString("  ")
			out.WriteString(f.String())
			out.WriteString("\n")
		}
	}
	out.WriteString("}")
	return out.String()
}

// StructField represents a typed field within a struct declaration.
type StructField struct {
	Token          token.Token     // Identifier token of the field
	Name           string          // Field name
	TypeAnnotation *TypeAnnotation // Field type annotation
	DefaultValue   Expression      // Default field value, or nil
}

func (sf *StructField) statementNode() {}

// TokenLiteral returns the literal field name token.
func (sf *StructField) TokenLiteral() string { return sf.Token.Literal }

// String returns the string representation of the struct field.
func (sf *StructField) String() string {
	var out strings.Builder
	out.WriteString(sf.Name)
	if sf.TypeAnnotation != nil {
		out.WriteString(": ")
		out.WriteString(sf.TypeAnnotation.String())
	}
	if sf.DefaultValue != nil {
		out.WriteString(" = ")
		out.WriteString(sf.DefaultValue.String())
	}
	return out.String()
}

// IfStatement represents an if-elif-else conditional branching statement.
type IfStatement struct {
	Token        token.Token     // token.IF
	Condition    Expression      // Primary conditional expression
	Consequence  *BlockStatement // Consequent body executed when condition evaluates to true
	Alternatives []*ElifClause   // Optional elif branches
	ElseBlock    *BlockStatement // Optional else body executed when all conditions fail
}

func (is *IfStatement) statementNode() {}

// TokenLiteral returns the literal representation of the if token.
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }

// String returns the string representation of the if-elif-else statement.
func (is *IfStatement) String() string {
	var out strings.Builder
	out.WriteString("if ")
	if is.Condition != nil {
		out.WriteString(is.Condition.String())
	}
	if is.Consequence != nil {
		out.WriteString(" ")
		out.WriteString(is.Consequence.String())
	}
	for _, alt := range is.Alternatives {
		if alt != nil {
			out.WriteString(" ")
			out.WriteString(alt.String())
		}
	}
	if is.ElseBlock != nil {
		out.WriteString(" else ")
		out.WriteString(is.ElseBlock.String())
	}
	return out.String()
}

// ElifClause represents an intermediate elif branch within an if statement.
type ElifClause struct {
	Token     token.Token     // token.ELIF
	Condition Expression      // Branch condition
	Body      *BlockStatement // Branch body
}

func (ec *ElifClause) statementNode() {}

// TokenLiteral returns the literal representation of the elif token.
func (ec *ElifClause) TokenLiteral() string { return ec.Token.Literal }

// String returns the string representation of the elif branch.
func (ec *ElifClause) String() string {
	var out strings.Builder
	out.WriteString("elif ")
	if ec.Condition != nil {
		out.WriteString(ec.Condition.String())
	}
	if ec.Body != nil {
		out.WriteString(" ")
		out.WriteString(ec.Body.String())
	}
	return out.String()
}

// WhileStatement represents a while loop statement.
type WhileStatement struct {
	Token     token.Token     // token.WHILE
	Condition Expression      // Continuation condition
	Body      *BlockStatement // Loop body
}

func (ws *WhileStatement) statementNode() {}

// TokenLiteral returns the literal representation of the while token.
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }

// String returns the string representation of the while loop.
func (ws *WhileStatement) String() string {
	var out strings.Builder
	out.WriteString("while ")
	if ws.Condition != nil {
		out.WriteString(ws.Condition.String())
	}
	if ws.Body != nil {
		out.WriteString(" ")
		out.WriteString(ws.Body.String())
	}
	return out.String()
}

// ForStatement represents an iterative for-in loop statement.
type ForStatement struct {
	Token    token.Token     // token.FOR
	Variable string          // Iteration variable name
	Iterable Expression      // Target collection or range to iterate over
	Body     *BlockStatement // Loop body
}

func (fs *ForStatement) statementNode() {}

// TokenLiteral returns the literal representation of the for token.
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }

// String returns the string representation of the for loop.
func (fs *ForStatement) String() string {
	var out strings.Builder
	out.WriteString("for ")
	out.WriteString(fs.Variable)
	out.WriteString(" in ")
	if fs.Iterable != nil {
		out.WriteString(fs.Iterable.String())
	}
	if fs.Body != nil {
		out.WriteString(" ")
		out.WriteString(fs.Body.String())
	}
	return out.String()
}

// ForEachStatement represents an explicit foreach-in loop statement.
type ForEachStatement struct {
	Token    token.Token     // token.FOREACH
	Variable string          // Iteration variable name
	Iterable Expression      // Target collection or range to iterate over
	Body     *BlockStatement // Loop body
}

func (fes *ForEachStatement) statementNode() {}

// TokenLiteral returns the literal representation of the foreach token.
func (fes *ForEachStatement) TokenLiteral() string { return fes.Token.Literal }

// String returns the string representation of the foreach loop.
func (fes *ForEachStatement) String() string {
	var out strings.Builder
	out.WriteString("foreach ")
	out.WriteString(fes.Variable)
	out.WriteString(" in ")
	if fes.Iterable != nil {
		out.WriteString(fes.Iterable.String())
	}
	if fes.Body != nil {
		out.WriteString(" ")
		out.WriteString(fes.Body.String())
	}
	return out.String()
}

// LoopStatement represents an unconditional infinite loop statement.
type LoopStatement struct {
	Token token.Token     // token.LOOP
	Body  *BlockStatement // Loop body executed repeatedly until break
}

func (ls *LoopStatement) statementNode() {}

// TokenLiteral returns the literal representation of the loop token.
func (ls *LoopStatement) TokenLiteral() string { return ls.Token.Literal }

// String returns the string representation of the loop statement.
func (ls *LoopStatement) String() string {
	var out strings.Builder
	out.WriteString("loop")
	if ls.Body != nil {
		out.WriteString(" ")
		out.WriteString(ls.Body.String())
	}
	return out.String()
}

// BlockStatement represents a grouped sequence of statements.
type BlockStatement struct {
	Token      token.Token // Left brace or indent token
	Statements []Statement // Sequence of contained statements
}

func (bs *BlockStatement) statementNode() {}

// TokenLiteral returns the literal representation of the block token.
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }

// String returns the string representation of the block statement enclosed in braces.
func (bs *BlockStatement) String() string {
	var out strings.Builder
	out.WriteString("{\n")
	for _, s := range bs.Statements {
		if s != nil {
			out.WriteString("  ")
			out.WriteString(s.String())
			out.WriteString("\n")
		}
	}
	out.WriteString("}")
	return out.String()
}

// ReturnStatement represents a return statement with an optional value expression.
type ReturnStatement struct {
	Token token.Token // token.RETURN
	Value Expression  // Value to return, or nil for bare return
}

func (rs *ReturnStatement) statementNode() {}

// TokenLiteral returns the literal representation of the return token.
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }

// String returns the string representation of the return statement.
func (rs *ReturnStatement) String() string {
	var out strings.Builder
	out.WriteString("return")
	if rs.Value != nil {
		out.WriteString(" ")
		out.WriteString(rs.Value.String())
	}
	return out.String()
}

// BreakStatement represents a loop break statement.
type BreakStatement struct {
	Token token.Token // token.BREAK
}

func (bs *BreakStatement) statementNode() {}

// TokenLiteral returns the literal representation of the break token.
func (bs *BreakStatement) TokenLiteral() string { return bs.Token.Literal }

// String returns the string representation of the break statement.
func (bs *BreakStatement) String() string { return "break" }

// ContinueStatement represents a loop continue statement.
type ContinueStatement struct {
	Token token.Token // token.CONTINUE
}

func (cs *ContinueStatement) statementNode() {}

// TokenLiteral returns the literal representation of the continue token.
func (cs *ContinueStatement) TokenLiteral() string { return cs.Token.Literal }

// String returns the string representation of the continue statement.
func (cs *ContinueStatement) String() string { return "continue" }

// ExpressionStatement wraps an expression to allow it to be evaluated as a statement.
type ExpressionStatement struct {
	Token      token.Token // First token of expression
	Expression Expression  // Wrapped expression
}

func (es *ExpressionStatement) statementNode() {}

// TokenLiteral returns the literal representation of the expression statement token.
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }

// String returns the string representation of the expression statement.
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

// UseStatement represents a module import or use declaration.
type UseStatement struct {
	Token      token.Token // token.USE or token.IMPORT
	ModulePath string      // Path or identifier of the module
	Imports    []string    // Explicitly imported symbols, or empty for all
	Alias      string      // Optional alias name (as alias)
}

func (us *UseStatement) statementNode() {}

// TokenLiteral returns the literal representation of the use/import token.
func (us *UseStatement) TokenLiteral() string { return us.Token.Literal }

// String returns the string representation of the use statement.
func (us *UseStatement) String() string {
	var out strings.Builder
	out.WriteString("use ")
	out.WriteString(us.ModulePath)
	if len(us.Imports) > 0 {
		out.WriteString(" [")
		out.WriteString(strings.Join(us.Imports, ", "))
		out.WriteString("]")
	}
	if us.Alias != "" {
		out.WriteString(" as ")
		out.WriteString(us.Alias)
	}
	return out.String()
}

// WatchStatement represents a reactive file or resource watcher block.
type WatchStatement struct {
	Token  token.Token     // token.WATCH
	Target Expression      // Target file, folder, or resource expression
	Body   *BlockStatement // Execution body triggered on resource changes
}

func (ws *WatchStatement) statementNode() {}

// TokenLiteral returns the literal representation of the watch token.
func (ws *WatchStatement) TokenLiteral() string { return ws.Token.Literal }

// String returns the string representation of the watch statement.
func (ws *WatchStatement) String() string {
	var out strings.Builder
	out.WriteString("watch ")
	if ws.Target != nil {
		out.WriteString(ws.Target.String())
	}
	if ws.Body != nil {
		out.WriteString(" ")
		out.WriteString(ws.Body.String())
	}
	return out.String()
}

// TestStatement represents a test case or test suite block.
type TestStatement struct {
	Token token.Token     // token.TEST
	Name  string          // Name or description of the test case
	Body  *BlockStatement // Test execution body
}

func (ts *TestStatement) statementNode() {}

// TokenLiteral returns the literal representation of the test token.
func (ts *TestStatement) TokenLiteral() string { return ts.Token.Literal }

// String returns the string representation of the test statement.
func (ts *TestStatement) String() string {
	var out strings.Builder
	out.WriteString("test ")
	out.WriteString(fmt.Sprintf("%q", ts.Name))
	if ts.Body != nil {
		out.WriteString(" ")
		out.WriteString(ts.Body.String())
	}
	return out.String()
}

// MockStatement represents a mock specification substituting a target expression.
type MockStatement struct {
	Token       token.Token // token.MOCK
	Target      Expression  // Expression to be mocked
	Replacement Expression  // Replacement mock expression
}

func (ms *MockStatement) statementNode() {}

// TokenLiteral returns the literal representation of the mock token.
func (ms *MockStatement) TokenLiteral() string { return ms.Token.Literal }

// String returns the string representation of the mock statement.
func (ms *MockStatement) String() string {
	var out strings.Builder
	out.WriteString("mock ")
	if ms.Target != nil {
		out.WriteString(ms.Target.String())
	}
	if ms.Replacement != nil {
		out.WriteString(" = ")
		out.WriteString(ms.Replacement.String())
	}
	return out.String()
}

// BaStatement represents a background asynchronous execution block.
type BaStatement struct {
	Token    token.Token     // token.BA
	Body     *BlockStatement // Asynchronous execution block
	Timeout  Expression      // Optional execution timeout expression (~ 5s)
	MemLimit string          // Optional memory limit string (e.g., 512MB)
	Limit    Expression      // Optional concurrency limit expression
}

func (bs *BaStatement) statementNode() {}

// TokenLiteral returns the literal representation of the ba token.
func (bs *BaStatement) TokenLiteral() string { return bs.Token.Literal }

// String returns the string representation of the ba statement.
func (bs *BaStatement) String() string {
	var out strings.Builder
	out.WriteString("ba")
	if bs.Timeout != nil {
		out.WriteString(" ~ ")
		out.WriteString(bs.Timeout.String())
	}
	if bs.MemLimit != "" {
		out.WriteString(" mem=")
		out.WriteString(bs.MemLimit)
	}
	if bs.Limit != nil {
		out.WriteString(" limit=")
		out.WriteString(bs.Limit.String())
	}
	if bs.Body != nil {
		out.WriteString(" ")
		out.WriteString(bs.Body.String())
	}
	return out.String()
}

// BaForStatement represents a parallel for-loop running concurrently.
type BaForStatement struct {
	Token    token.Token     // token.BA
	Variable string          // Iteration variable name
	Iterable Expression      // Target collection or range
	Body     *BlockStatement // Concurrently executed loop body
	Limit    Expression      // Optional concurrency limit expression
}

func (bfs *BaForStatement) statementNode() {}

// TokenLiteral returns the literal representation of the ba token.
func (bfs *BaForStatement) TokenLiteral() string { return bfs.Token.Literal }

// String returns the string representation of the ba for loop statement.
func (bfs *BaForStatement) String() string {
	var out strings.Builder
	out.WriteString("ba for ")
	out.WriteString(bfs.Variable)
	out.WriteString(" in ")
	if bfs.Iterable != nil {
		out.WriteString(bfs.Iterable.String())
	}
	if bfs.Limit != nil {
		out.WriteString(" limit=")
		out.WriteString(bfs.Limit.String())
	}
	if bfs.Body != nil {
		out.WriteString(" ")
		out.WriteString(bfs.Body.String())
	}
	return out.String()
}

// ThreadsStatement represents an OS-thread bound block of statements.
type ThreadsStatement struct {
	Token token.Token     // token.THREADS
	Body  *BlockStatement // Threaded execution body
}

func (ts *ThreadsStatement) statementNode() {}

// TokenLiteral returns the literal representation of the threads token.
func (ts *ThreadsStatement) TokenLiteral() string { return ts.Token.Literal }

// String returns the string representation of the threads statement.
func (ts *ThreadsStatement) String() string {
	var out strings.Builder
	out.WriteString("threads")
	if ts.Body != nil {
		out.WriteString(" ")
		out.WriteString(ts.Body.String())
	}
	return out.String()
}

// TxStatement represents an atomic transactional block with an optional timeout.
type TxStatement struct {
	Token   token.Token     // token.TX
	DB      Expression      // Database connection or context expression
	Timeout Expression      // Optional transaction timeout expression
	Body    *BlockStatement // Transaction body
}

func (ts *TxStatement) statementNode() {}

// TokenLiteral returns the literal representation of the tx token.
func (ts *TxStatement) TokenLiteral() string { return ts.Token.Literal }

// String returns the string representation of the tx statement.
func (ts *TxStatement) String() string {
	var out strings.Builder
	out.WriteString("tx")
	if ts.DB != nil {
		out.WriteString(" ")
		out.WriteString(ts.DB.String())
	}
	if ts.Timeout != nil {
		out.WriteString(" ~ ")
		out.WriteString(ts.Timeout.String())
	}
	if ts.Body != nil {
		out.WriteString(" ")
		out.WriteString(ts.Body.String())
	}
	return out.String()
}

// WaitStatement represents a synchronization barrier waiting for background tasks.
type WaitStatement struct {
	Token token.Token // token.WAIT
}

func (ws *WaitStatement) statementNode() {}

// TokenLiteral returns the literal representation of the wait token.
func (ws *WaitStatement) TokenLiteral() string { return ws.Token.Literal }

// String returns the string representation of the wait statement.
func (ws *WaitStatement) String() string { return "wait" }

// AssertStatement represents an assertion statement for test verification.
type AssertStatement struct {
	Token      token.Token // token.ASSERT
	Expression Expression  // Asserted condition expression
}

func (as *AssertStatement) statementNode() {}

// TokenLiteral returns the literal representation of the assert token.
func (as *AssertStatement) TokenLiteral() string { return as.Token.Literal }

// String returns the string representation of the assert statement.
func (as *AssertStatement) String() string {
	var out strings.Builder
	out.WriteString("assert ")
	if as.Expression != nil {
		out.WriteString(as.Expression.String())
	}
	return out.String()
}

// Decorator represents an annotation attached to functions, structs, or variables (@name(args)).
type Decorator struct {
	Token token.Token  // token.AT
	Name  string       // Decorator identifier name
	Args  []Expression // Optional decorator arguments
}

func (d *Decorator) statementNode() {}

// TokenLiteral returns the literal representation of the decorator prefix token.
func (d *Decorator) TokenLiteral() string { return d.Token.Literal }

// String returns the string representation of the decorator annotation.
func (d *Decorator) String() string {
	var out strings.Builder
	out.WriteString("@")
	out.WriteString(d.Name)
	if len(d.Args) > 0 {
		out.WriteString("(")
		args := make([]string, 0, len(d.Args))
		for _, a := range d.Args {
			if a != nil {
				args = append(args, a.String())
			}
		}
		out.WriteString(strings.Join(args, ", "))
		out.WriteString(")")
	}
	return out.String()
}

// MatchStatement represents a pattern matching control flow statement.
type MatchStatement struct {
	Token   token.Token     // token.MATCH
	Subject Expression      // Expression being matched
	Cases   []*MatchCase    // Pattern matching cases
	Default *BlockStatement // Optional default block when no patterns match
}

func (ms *MatchStatement) statementNode() {}

// TokenLiteral returns the literal representation of the match token.
func (ms *MatchStatement) TokenLiteral() string { return ms.Token.Literal }

// String returns the string representation of the match statement.
func (ms *MatchStatement) String() string {
	var out strings.Builder
	out.WriteString("match ")
	if ms.Subject != nil {
		out.WriteString(ms.Subject.String())
	}
	out.WriteString(" {\n")
	for _, c := range ms.Cases {
		if c != nil {
			out.WriteString("  ")
			out.WriteString(c.String())
			out.WriteString("\n")
		}
	}
	if ms.Default != nil {
		out.WriteString("  default ")
		out.WriteString(ms.Default.String())
		out.WriteString("\n")
	}
	out.WriteString("}")
	return out.String()
}

// MatchCase represents an individual case arm in a match statement.
type MatchCase struct {
	Token   token.Token     // token.CASE
	Pattern Expression      // Target match pattern expression
	Body    *BlockStatement // Execution body for this match case
}

func (mc *MatchCase) statementNode() {}

// TokenLiteral returns the literal representation of the case token.
func (mc *MatchCase) TokenLiteral() string { return mc.Token.Literal }

// String returns the string representation of the match case.
func (mc *MatchCase) String() string {
	var out strings.Builder
	out.WriteString("case ")
	if mc.Pattern != nil {
		out.WriteString(mc.Pattern.String())
	}
	if mc.Body != nil {
		out.WriteString(" ")
		out.WriteString(mc.Body.String())
	}
	return out.String()
}

// -----------------------------------------------------------------------------
// Expression Nodes
// -----------------------------------------------------------------------------

// Identifier represents a named identifier expression.
type Identifier struct {
	Token token.Token // token.IDENT
	Value string      // Identifier name
}

func (i *Identifier) expressionNode() {}

// TokenLiteral returns the literal representation of the identifier token.
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }

// String returns the string representation of the identifier.
func (i *Identifier) String() string { return i.Value }

// IntegerLiteral represents an integer constant expression.
type IntegerLiteral struct {
	Token token.Token // token.INT
	Value int64       // Parsed 64-bit integer value
}

func (il *IntegerLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the integer token.
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }

// String returns the string representation of the integer literal.
func (il *IntegerLiteral) String() string {
	if il.Token.Literal != "" {
		return il.Token.Literal
	}
	return fmt.Sprintf("%d", il.Value)
}

// FloatLiteral represents a floating-point constant expression.
type FloatLiteral struct {
	Token token.Token // token.FLOAT
	Value float64     // Parsed 64-bit float value
}

func (fl *FloatLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the float token.
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }

// String returns the string representation of the float literal.
func (fl *FloatLiteral) String() string {
	if fl.Token.Literal != "" {
		return fl.Token.Literal
	}
	return fmt.Sprintf("%g", fl.Value)
}

// StringLiteral represents a string constant expression with interpolated segments resolved.
type StringLiteral struct {
	Token token.Token // token.STRING
	Value string      // String content
}

func (sl *StringLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the string token.
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }

// String returns the string representation of the string literal with surrounding quotes.
func (sl *StringLiteral) String() string {
	return fmt.Sprintf("%q", sl.Value)
}

// BooleanLiteral represents a boolean constant (true or false).
type BooleanLiteral struct {
	Token token.Token // token.TRUE or token.FALSE
	Value bool        // Boolean truth value
}

func (bl *BooleanLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the boolean token.
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }

// String returns the string representation of the boolean literal.
func (bl *BooleanLiteral) String() string {
	return fmt.Sprintf("%t", bl.Value)
}

// NullLiteral represents the null value literal.
type NullLiteral struct {
	Token token.Token // token.NULL
}

func (nl *NullLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the null token.
func (nl *NullLiteral) TokenLiteral() string { return nl.Token.Literal }

// String returns the string representation of null.
func (nl *NullLiteral) String() string { return "null" }

// ListLiteral represents a sequence of expressions enclosed in brackets.
type ListLiteral struct {
	Token    token.Token  // token.LBRACKET
	Elements []Expression // List item expressions
}

func (ll *ListLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the opening bracket token.
func (ll *ListLiteral) TokenLiteral() string { return ll.Token.Literal }

// String returns the string representation of the list literal.
func (ll *ListLiteral) String() string {
	var out strings.Builder
	out.WriteString("[")
	elems := make([]string, 0, len(ll.Elements))
	for _, el := range ll.Elements {
		if el != nil {
			elems = append(elems, el.String())
		}
	}
	out.WriteString(strings.Join(elems, ", "))
	out.WriteString("]")
	return out.String()
}

// MapPair represents a key-value pair within a map literal.
type MapPair struct {
	Token token.Token // Colon or opening delimiter token
	Key   Expression  // Key expression
	Value Expression  // Value expression
}

// TokenLiteral returns the literal representation of the map pair token.
func (mp *MapPair) TokenLiteral() string { return mp.Token.Literal }

// String returns the string representation of the key-value pair.
func (mp *MapPair) String() string {
	var k, v string
	if mp.Key != nil {
		k = mp.Key.String()
	}
	if mp.Value != nil {
		v = mp.Value.String()
	}
	return fmt.Sprintf("%s: %s", k, v)
}

// MapLiteral represents a dictionary/map literal enclosed in braces.
type MapLiteral struct {
	Token token.Token // token.LBRACE
	Pairs []*MapPair  // Map key-value pairs
}

func (ml *MapLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the opening brace token.
func (ml *MapLiteral) TokenLiteral() string { return ml.Token.Literal }

// String returns the string representation of the map literal.
func (ml *MapLiteral) String() string {
	var out strings.Builder
	out.WriteString("{")
	pairs := make([]string, 0, len(ml.Pairs))
	for _, p := range ml.Pairs {
		if p != nil {
			pairs = append(pairs, p.String())
		}
	}
	out.WriteString(strings.Join(pairs, ", "))
	out.WriteString("}")
	return out.String()
}

// PrefixExpression represents a unary prefix operation (e.g. -x, not flag).
type PrefixExpression struct {
	Token    token.Token // Operator token
	Operator string      // Operator symbol or keyword (-, not, etc.)
	Right    Expression  // Right-hand operand
}

func (pe *PrefixExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the prefix operator token.
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }

// String returns the string representation of the prefix expression.
func (pe *PrefixExpression) String() string {
	var out strings.Builder
	out.WriteString("(")
	out.WriteString(pe.Operator)
	if pe.Operator == "not" {
		out.WriteString(" ")
	}
	if pe.Right != nil {
		out.WriteString(pe.Right.String())
	}
	out.WriteString(")")
	return out.String()
}

// InfixExpression represents a binary operation with an operator between operands.
type InfixExpression struct {
	Token    token.Token // Infix operator token
	Left     Expression  // Left operand
	Operator string      // Infix operator string
	Right    Expression  // Right operand
}

func (ie *InfixExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the infix operator token.
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }

// String returns the string representation of the infix expression.
func (ie *InfixExpression) String() string {
	var out strings.Builder
	out.WriteString("(")
	if ie.Left != nil {
		out.WriteString(ie.Left.String())
	}
	out.WriteString(" ")
	out.WriteString(ie.Operator)
	out.WriteString(" ")
	if ie.Right != nil {
		out.WriteString(ie.Right.String())
	}
	out.WriteString(")")
	return out.String()
}

// CallExpression represents a function or method invocation.
type CallExpression struct {
	Token     token.Token           // Left parenthesis or call token
	Function  Expression            // Function expression being invoked
	Arguments []Expression          // Positional arguments
	KwArgs    map[string]Expression // Named keyword arguments
}

func (ce *CallExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the call token.
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }

// String returns the string representation of the call expression.
func (ce *CallExpression) String() string {
	var out strings.Builder
	if ce.Function != nil {
		out.WriteString(ce.Function.String())
	}
	out.WriteString("(")
	args := make([]string, 0, len(ce.Arguments)+len(ce.KwArgs))
	for _, a := range ce.Arguments {
		if a != nil {
			args = append(args, a.String())
		}
	}
	if len(ce.KwArgs) > 0 {
		keys := make([]string, 0, len(ce.KwArgs))
		for k := range ce.KwArgs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			valStr := ""
			if ce.KwArgs[k] != nil {
				valStr = ce.KwArgs[k].String()
			}
			args = append(args, fmt.Sprintf("%s=%s", k, valStr))
		}
	}
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

// IndexExpression represents indexing into a collection or string (left[index]).
type IndexExpression struct {
	Token token.Token // token.LBRACKET
	Left  Expression  // Indexed collection expression
	Index Expression  // Index or subscript expression
}

func (ie *IndexExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the index token.
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }

// String returns the string representation of the index expression.
func (ie *IndexExpression) String() string {
	var out strings.Builder
	out.WriteString("(")
	if ie.Left != nil {
		out.WriteString(ie.Left.String())
	}
	out.WriteString("[")
	if ie.Index != nil {
		out.WriteString(ie.Index.String())
	}
	out.WriteString("])")
	return out.String()
}

// DotExpression represents member field or method access (left.field).
type DotExpression struct {
	Token token.Token // token.DOT
	Left  Expression  // Target operand
	Field string      // Field or attribute name
}

func (de *DotExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the dot token.
func (de *DotExpression) TokenLiteral() string { return de.Token.Literal }

// String returns the string representation of the dot access expression.
func (de *DotExpression) String() string {
	var out strings.Builder
	if de.Left != nil {
		out.WriteString(de.Left.String())
	}
	out.WriteString(".")
	out.WriteString(de.Field)
	return out.String()
}

// AssignExpression represents an assignment expression (target = value).
type AssignExpression struct {
	Token  token.Token // token.ASSIGN
	Target Expression  // Target receiving the value
	Value  Expression  // Value expression being assigned
}

func (ae *AssignExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the assignment token.
func (ae *AssignExpression) TokenLiteral() string { return ae.Token.Literal }

// String returns the string representation of the assignment expression.
func (ae *AssignExpression) String() string {
	var out strings.Builder
	if ae.Target != nil {
		out.WriteString(ae.Target.String())
	}
	out.WriteString(" = ")
	if ae.Value != nil {
		out.WriteString(ae.Value.String())
	}
	return out.String()
}

// StreamExpression represents stream redirection (>>, >!, >>>, <<, ?>>, ?>>>, &>>, &&>>).
type StreamExpression struct {
	Token    token.Token // Stream operator token
	Source   Expression  // Source stream or command expression
	Operator string      // Redirection operator string
	Target   Expression  // Target destination expression
}

func (se *StreamExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the stream operator token.
func (se *StreamExpression) TokenLiteral() string { return se.Token.Literal }

// String returns the string representation of the stream expression.
func (se *StreamExpression) String() string {
	var out strings.Builder
	if se.Source != nil {
		out.WriteString(se.Source.String())
	}
	out.WriteString(" ")
	out.WriteString(se.Operator)
	out.WriteString(" ")
	if se.Target != nil {
		out.WriteString(se.Target.String())
	}
	return out.String()
}

// PipeExpression represents pipeline chaining (left |> right).
type PipeExpression struct {
	Token token.Token // token.PIPE_ARROW
	Left  Expression  // Data producer expression
	Right Expression  // Consumer expression or function
}

func (pe *PipeExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the pipe arrow token.
func (pe *PipeExpression) TokenLiteral() string { return pe.Token.Literal }

// String returns the string representation of the pipe expression.
func (pe *PipeExpression) String() string {
	var out strings.Builder
	if pe.Left != nil {
		out.WriteString(pe.Left.String())
	}
	out.WriteString(" |> ")
	if pe.Right != nil {
		out.WriteString(pe.Right.String())
	}
	return out.String()
}

// TimeoutExpression represents an expression bound by a duration timeout (~ duration).
type TimeoutExpression struct {
	Token      token.Token // token.TILDE
	Expression Expression  // Target expression under timeout
	Expr       Expression  // Target expression alias
	Duration   Expression  // Duration expression (e.g. 5s, 500ms)
}

func (te *TimeoutExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the tilde timeout token.
func (te *TimeoutExpression) TokenLiteral() string { return te.Token.Literal }

// String returns the string representation of the timeout expression.
func (te *TimeoutExpression) String() string {
	var out strings.Builder
	if te.Expression != nil {
		out.WriteString(te.Expression.String())
	}
	out.WriteString(" ~ ")
	if te.Duration != nil {
		out.WriteString(te.Duration.String())
	}
	return out.String()
}

// ConsoleLiteral represents an interactive console input prompt $(prompt).
type ConsoleLiteral struct {
	Token  token.Token // Console literal token
	Prompt Expression  // Prompt message expression, or nil
}

func (cl *ConsoleLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the console token.
func (cl *ConsoleLiteral) TokenLiteral() string { return cl.Token.Literal }

// String returns the string representation of the console literal.
func (cl *ConsoleLiteral) String() string {
	var out strings.Builder
	out.WriteString("$(")
	if cl.Prompt != nil {
		out.WriteString(cl.Prompt.String())
	}
	out.WriteString(")")
	return out.String()
}

// LambdaExpression represents an anonymous function expression (params -> body).
type LambdaExpression struct {
	Token  token.Token  // Lambda delimiter or arrow token
	Params []*FuncParam // Lambda parameters
	Body   Expression   // Expression or block evaluated by lambda
}

func (le *LambdaExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the lambda token.
func (le *LambdaExpression) TokenLiteral() string { return le.Token.Literal }

// String returns the string representation of the lambda expression.
func (le *LambdaExpression) String() string {
	var out strings.Builder
	out.WriteString("(")
	params := make([]string, 0, len(le.Params))
	for _, p := range le.Params {
		if p != nil {
			params = append(params, p.String())
		}
	}
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") -> ")
	if le.Body != nil {
		out.WriteString(le.Body.String())
	}
	return out.String()
}

// TypeAnnotation represents type annotations, including union types (e.g. int | string).
type TypeAnnotation struct {
	Token token.Token // First type keyword token
	Types []string    // List of allowable types in union
}

func (ta *TypeAnnotation) expressionNode() {}

// TokenLiteral returns the literal representation of the type token.
func (ta *TypeAnnotation) TokenLiteral() string { return ta.Token.Literal }

// String returns the formatted union type string.
func (ta *TypeAnnotation) String() string {
	return strings.Join(ta.Types, " | ")
}

// ErrExpression represents an explicit error instantiation (err(message, code)).
type ErrExpression struct {
	Token   token.Token // token.ERR
	Message Expression  // Error message expression
	Code    Expression  // Optional error code expression, or nil
}

func (ee *ErrExpression) expressionNode() {}

// TokenLiteral returns the literal representation of the err token.
func (ee *ErrExpression) TokenLiteral() string { return ee.Token.Literal }

// String returns the string representation of the error expression.
func (ee *ErrExpression) String() string {
	var out strings.Builder
	out.WriteString("err(")
	if ee.Message != nil {
		out.WriteString(ee.Message.String())
	}
	if ee.Code != nil {
		out.WriteString(", code=")
		out.WriteString(ee.Code.String())
	}
	out.WriteString(")")
	return out.String()
}

// RangeLiteral represents an integer or sequential range expression (start..end).
type RangeLiteral struct {
	Token token.Token // token.RANGE ("..")
	Start Expression  // Starting bound expression
	End   Expression  // Ending bound expression
}

func (rl *RangeLiteral) expressionNode() {}

// TokenLiteral returns the literal representation of the range token.
func (rl *RangeLiteral) TokenLiteral() string { return rl.Token.Literal }

// String returns the string representation of the range literal.
func (rl *RangeLiteral) String() string {
	var out strings.Builder
	if rl.Start != nil {
		out.WriteString(rl.Start.String())
	}
	out.WriteString("..")
	if rl.End != nil {
		out.WriteString(rl.End.String())
	}
	return out.String()
}

// StructInstantiation represents an instance creation of a struct type (Name(args)).
type StructInstantiation struct {
	Token token.Token  // Struct identifier token
	Name  string       // Struct name
	Args  []Expression // Instantiation arguments
}

func (si *StructInstantiation) expressionNode() {}

// TokenLiteral returns the literal representation of the struct name token.
func (si *StructInstantiation) TokenLiteral() string { return si.Token.Literal }

// String returns the string representation of the struct instantiation.
func (si *StructInstantiation) String() string {
	var out strings.Builder
	out.WriteString(si.Name)
	out.WriteString("(")
	args := make([]string, 0, len(si.Args))
	for _, a := range si.Args {
		if a != nil {
			args = append(args, a.String())
		}
	}
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")
	return out.String()
}

// NodePosition extracts the 1-based source line and column of any AST node.
func NodePosition(node Node) (int, int) {
	if node == nil {
		return 0, 0
	}
	switch n := node.(type) {
	case *VarDeclaration:
		return n.Token.Line, n.Token.Column
	case *FuncDeclaration:
		return n.Token.Line, n.Token.Column
	case *StructDeclaration:
		return n.Token.Line, n.Token.Column
	case *IfStatement:
		return n.Token.Line, n.Token.Column
	case *WhileStatement:
		return n.Token.Line, n.Token.Column
	case *ForStatement:
		return n.Token.Line, n.Token.Column
	case *ForEachStatement:
		return n.Token.Line, n.Token.Column
	case *LoopStatement:
		return n.Token.Line, n.Token.Column
	case *BlockStatement:
		return n.Token.Line, n.Token.Column
	case *ReturnStatement:
		return n.Token.Line, n.Token.Column
	case *BreakStatement:
		return n.Token.Line, n.Token.Column
	case *ContinueStatement:
		return n.Token.Line, n.Token.Column
	case *ExpressionStatement:
		if n.Expression != nil {
			return NodePosition(n.Expression)
		}
		return n.Token.Line, n.Token.Column
	case *UseStatement:
		return n.Token.Line, n.Token.Column
	case *WatchStatement:
		return n.Token.Line, n.Token.Column
	case *TestStatement:
		return n.Token.Line, n.Token.Column
	case *MockStatement:
		return n.Token.Line, n.Token.Column
	case *BaStatement:
		return n.Token.Line, n.Token.Column
	case *BaForStatement:
		return n.Token.Line, n.Token.Column
	case *ThreadsStatement:
		return n.Token.Line, n.Token.Column
	case *TxStatement:
		return n.Token.Line, n.Token.Column
	case *WaitStatement:
		return n.Token.Line, n.Token.Column
	case *AssertStatement:
		return n.Token.Line, n.Token.Column
	case *MatchStatement:
		return n.Token.Line, n.Token.Column
	case *Identifier:
		return n.Token.Line, n.Token.Column
	case *IntegerLiteral:
		return n.Token.Line, n.Token.Column
	case *FloatLiteral:
		return n.Token.Line, n.Token.Column
	case *StringLiteral:
		return n.Token.Line, n.Token.Column
	case *BooleanLiteral:
		return n.Token.Line, n.Token.Column
	case *NullLiteral:
		return n.Token.Line, n.Token.Column
	case *ListLiteral:
		return n.Token.Line, n.Token.Column
	case *MapLiteral:
		return n.Token.Line, n.Token.Column
	case *PrefixExpression:
		return n.Token.Line, n.Token.Column
	case *InfixExpression:
		return n.Token.Line, n.Token.Column
	case *CallExpression:
		return n.Token.Line, n.Token.Column
	case *IndexExpression:
		return n.Token.Line, n.Token.Column
	case *DotExpression:
		return n.Token.Line, n.Token.Column
	case *AssignExpression:
		return n.Token.Line, n.Token.Column
	case *StreamExpression:
		return n.Token.Line, n.Token.Column
	case *PipeExpression:
		return n.Token.Line, n.Token.Column
	case *TimeoutExpression:
		return n.Token.Line, n.Token.Column
	case *ConsoleLiteral:
		return n.Token.Line, n.Token.Column
	case *LambdaExpression:
		return n.Token.Line, n.Token.Column
	case *ErrExpression:
		return n.Token.Line, n.Token.Column
	case *RangeLiteral:
		return n.Token.Line, n.Token.Column
	case *StructInstantiation:
		return n.Token.Line, n.Token.Column
	}
	return 0, 0
}
