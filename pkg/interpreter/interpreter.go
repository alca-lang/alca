// Package interpreter implements the tree-walking runtime engine for the Alca programming language.
package interpreter

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
)

// ModuleParser is a function type that parses source code into an AST Program.
type ModuleParser func(content string) (*ast.Program, error)

var (
	moduleParserMu     sync.RWMutex
	activeModuleParser ModuleParser
)

// SetModuleParser registers a source code parser to support importing external .al files.
func SetModuleParser(mp ModuleParser) {
	moduleParserMu.Lock()
	defer moduleParserMu.Unlock()
	activeModuleParser = mp
}

// Interpreter is the core tree-walking evaluation engine for Alca.
type Interpreter struct {
	env         *Environment
	globals     *Environment
	errors      []ErrorValue
	errorLog    string           // global error log path from @error_log
	oomMax      int64            // max bytes from @oom_guard
	oomPolicy   string           // oom policy strategy (e.g. "heaviest")
	memUsed     int64            // approximate memory tracking
	mocks       map[string]Value // for test mocking
	dropOnOOM   map[string]bool  // variables marked with @drop_on_oom
	cache       map[string]Value // memoization cache for @cache
	filename    string           // active source file being executed
	source      string           // full source text for error reporting
	currentNode ast.Node         // currently executing AST node
	mu          sync.RWMutex
}

// SetFilename sets the active filename for diagnostic reporting.
func (i *Interpreter) SetFilename(filename string) {
	i.filename = filename
}

// SetSource sets the full source code for diagnostic context and error previews.
func (i *Interpreter) SetSource(source string) {
	i.source = source
}

// New creates and initializes a new Alca interpreter instance with built-in functions.
func New() *Interpreter {
	globals := NewEnvironment()
	for name, fn := range BuiltinFunctionMap {
		globals.Set(name, fn)
	}

	interp := &Interpreter{
		env:       globals,
		globals:   globals,
		errors:    make([]ErrorValue, 0),
		mocks:     make(map[string]Value),
		dropOnOOM: make(map[string]bool),
		cache:     make(map[string]Value),
	}

	// Register evaluation callback for higher-order method execution.
	SetMethodEvaluator(func(fn Value, args []Value) (Value, error) {
		return interp.invokeFunction(fn, args, nil)
	})

	return interp
}

// Env returns the interpreter's current environment.
func (i *Interpreter) Env() *Environment {
	return i.env
}

// Globals returns the interpreter's global environment.
func (i *Interpreter) Globals() *Environment {
	return i.globals
}

// Fork creates an isolated child interpreter inheriting configurations and globals.
func (i *Interpreter) Fork(env *Environment) *Interpreter {
	i.mu.RLock()
	defer i.mu.RUnlock()

	return &Interpreter{
		env:       env,
		globals:   i.globals,
		errors:    make([]ErrorValue, 0),
		errorLog:  i.errorLog,
		oomMax:    i.oomMax,
		oomPolicy: i.oomPolicy,
		memUsed:   i.memUsed,
		mocks:     i.mocks,
		dropOnOOM: i.dropOnOOM,
		cache:     i.cache,
	}
}

// Execute evaluates a parsed program in the interpreter's current environment.
func (i *Interpreter) Execute(program *ast.Program) error {
	return i.ExecuteInEnv(program, i.env)
}

// ExecuteInEnv executes a parsed program within a specific environment scope.
func (i *Interpreter) ExecuteInEnv(program *ast.Program, env *Environment) error {
	if program == nil {
		return nil
	}

	oldEnv := i.env
	i.env = env
	defer func() {
		i.env = oldEnv
	}()

	for _, stmt := range hoistDeclarations(program.Statements) {
		if stmt == nil {
			continue
		}
		i.currentNode = stmt
		res := i.Eval(stmt)
		if res != nil {
			if errVal, ok := res.(*ErrorValue); ok {
				i.recordError(errVal)
				line, col := ast.NodePosition(i.currentNode)
				if line == 0 {
					line, col = ast.NodePosition(stmt)
				}
				return &diag.RuntimeError{
					Filename: i.filename,
					Line:     line,
					Column:   col,
					Message:  errVal.Message,
					Code:     errVal.Code,
				}
			}
			if ret, ok := res.(*ReturnValue); ok {
				if errVal, ok := ret.Val.(*ErrorValue); ok {
					i.recordError(errVal)
					line, col := ast.NodePosition(i.currentNode)
					if line == 0 {
						line, col = ast.NodePosition(stmt)
					}
					return &diag.RuntimeError{
						Filename: i.filename,
						Line:     line,
						Column:   col,
						Message:  errVal.Message,
						Code:     errVal.Code,
					}
				}
			}
		}
	}
	return nil
}

// recordError appends an error to the error collection and logs to errorLog if configured.
func (i *Interpreter) recordError(errVal *ErrorValue) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.errors = append(i.errors, *errVal)
	if i.errorLog != "" {
		logEntry := fmt.Sprintf("[%s] Error(%d): %s\n", errVal.Time, errVal.Code, errVal.Message)
		_ = WriteFile(i.errorLog, logEntry, true)
	}
}

// trackMemory updates memory usage estimates and triggers OOM policies if threshold exceeded.
func (i *Interpreter) trackMemory(delta int64) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.memUsed += delta
	if i.oomMax > 0 && i.memUsed > i.oomMax {
		// Trigger OOM mitigation
		for varName := range i.dropOnOOM {
			i.env.Delete(varName)
		}
		// Reset approximate counter
		i.memUsed = i.oomMax / 2
	}
}

// Eval is the central evaluation dispatcher that walks any AST node and computes its runtime Value.
func (i *Interpreter) Eval(node ast.Node) Value {
	if node == nil {
		return NULL
	}

	switch n := node.(type) {
	// -------------------------------------------------------------------------
	// Program & Statements
	// -------------------------------------------------------------------------
	case *ast.Program:
		return i.evalProgram(n)

	case *ast.VarDeclaration:
		return i.evalVarDeclaration(n)

	case *ast.FuncDeclaration:
		return i.evalFuncDeclaration(n)

	case *ast.StructDeclaration:
		return i.evalStructDeclaration(n)

	case *ast.IfStatement:
		return i.evalIfStatement(n)

	case *ast.WhileStatement:
		return i.evalWhileStatement(n)

	case *ast.ForStatement:
		return i.evalForStatement(n)

	case *ast.ForEachStatement:
		return i.evalForEachStatement(n)

	case *ast.LoopStatement:
		return i.evalLoopStatement(n)

	case *ast.BlockStatement:
		return i.evalBlockStatement(n)

	case *ast.ReturnStatement:
		var val Value = NULL
		if n.Value != nil {
			val = i.Eval(n.Value)
		}
		return &ReturnValue{Val: val}

	case *ast.BreakStatement:
		return BREAK

	case *ast.ContinueStatement:
		return CONTINUE

	case *ast.ExpressionStatement:
		return i.Eval(n.Expression)

	case *ast.UseStatement:
		return i.evalUseStatement(n)

	case *ast.WatchStatement:
		return i.evalWatchStatement(n)

	case *ast.TestStatement:
		return i.evalTestStatement(n)

	case *ast.MockStatement:
		return i.evalMockStatement(n)

	case *ast.BaStatement:
		return i.evalBaStatement(n)

	case *ast.BaForStatement:
		return i.evalBaForStatement(n)

	case *ast.ThreadsStatement:
		return i.evalThreadsStatement(n)

	case *ast.WaitStatement:
		WaitAll()
		return NULL

	case *ast.AssertStatement:
		val := i.Eval(n.Expression)
		if !IsTruthy(val) {
			errVal := NewError(500, fmt.Sprintf("assertion failed: %s", n.Expression.String()))
			i.recordError(errVal)
			return errVal
		}
		return TRUE

	case *ast.MatchStatement:
		return i.evalMatchStatement(n)

	case *ast.TxStatement:
		return i.evalTxStatement(n)

	// -------------------------------------------------------------------------
	// Expressions
	// -------------------------------------------------------------------------
	case *ast.Identifier:
		return i.evalIdentifier(n)

	case *ast.IntegerLiteral:
		return &IntegerValue{Val: n.Value}

	case *ast.FloatLiteral:
		return &FloatValue{Val: n.Value}

	case *ast.StringLiteral:
		return &StringValue{Val: n.Value}

	case *ast.BooleanLiteral:
		if n.Value {
			return TRUE
		}
		return FALSE

	case *ast.NullLiteral:
		return NULL

	case *ast.ListLiteral:
		elems := make([]Value, len(n.Elements))
		for idx, el := range n.Elements {
			elems[idx] = i.Eval(el)
		}
		i.trackMemory(int64(len(elems) * 16))
		return NewList(elems...)

	case *ast.MapLiteral:
		m := NewMap()
		for _, pair := range n.Pairs {
			var keyStr string
			if ident, ok := pair.Key.(*ast.Identifier); ok {
				keyStr = ident.Value
			} else {
				kVal := i.Eval(pair.Key)
				keyStr = CoerceToString(kVal)
			}
			vVal := i.Eval(pair.Value)
			m.Set(keyStr, vVal)
		}
		i.trackMemory(int64(m.Len() * 32))
		return m

	case *ast.PrefixExpression:
		return i.evalPrefixExpression(n)

	case *ast.InfixExpression:
		return i.evalInfixExpression(n)

	case *ast.CallExpression:
		return i.evalCallExpression(n)

	case *ast.IndexExpression:
		return i.evalIndexExpression(n)

	case *ast.DotExpression:
		return i.evalDotExpression(n)

	case *ast.AssignExpression:
		return i.evalAssignExpression(n)

	case *ast.StreamExpression:
		var target Value
		if ident, ok := n.Target.(*ast.Identifier); ok && (n.Operator == ">>>" || n.Operator == "?>>>") {
			target = &StringValue{Val: ident.Value}
		} else {
			target = i.Eval(n.Target)
		}

		source := i.Eval(n.Source)
		res, err := HandleStream(source, n.Operator, target, i.env)
		if err != nil {
			errVal := NewError(500, err.Error())
			i.recordError(errVal)
			return errVal
		}

		// Intercepted errors (?>> or ?>>>) should not crash the program
		if n.Operator == "?>>" || n.Operator == "?>>>" {
			return NULL
		}

		return res

	case *ast.PipeExpression:
		return i.evalPipeExpression(n)

	case *ast.TimeoutExpression:
		return i.evalTimeoutExpression(n)

	case *ast.ConsoleLiteral:
		prompt := ""
		if n.Prompt != nil {
			prompt = CoerceToString(i.Eval(n.Prompt))
			fmt.Print(prompt)
		}
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return &StringValue{Val: scanner.Text()}
		}
		return &StringValue{Val: ""}

	case *ast.LambdaExpression:
		return &FuncValue{
			Name:   "",
			Params: n.Params,
			Body: &ast.BlockStatement{
				Statements: []ast.Statement{
					&ast.ReturnStatement{Value: n.Body},
				},
			},
			Env:     i.env,
			IsShell: false,
		}

	case *ast.ErrExpression:
		msg := "error"
		code := int64(500)
		if n.Message != nil {
			msg = CoerceToString(i.Eval(n.Message))
		}
		if n.Code != nil {
			cVal := i.Eval(n.Code)
			if iv, ok := cVal.(*IntegerValue); ok {
				code = iv.Val
			}
		}
		return NewError(code, msg)

	case *ast.RangeLiteral:
		startVal := i.Eval(n.Start)
		endVal := i.Eval(n.End)
		return evalRange(startVal, endVal)

	case *ast.StructInstantiation:
		defVal, ok := i.env.Get(n.Name)
		if !ok {
			return NewError(404, fmt.Sprintf("struct %s not defined", n.Name))
		}
		sDef, ok := defVal.(*StructDef)
		if !ok {
			return NewError(400, fmt.Sprintf("%s is not a struct", n.Name))
		}
		args := make([]Value, len(n.Args))
		for idx, a := range n.Args {
			args[idx] = i.Eval(a)
		}
		return i.instantiateStruct(sDef, args, nil)

	default:
		return NULL
	}
}

// -----------------------------------------------------------------------------
// Statement Evaluators
// -----------------------------------------------------------------------------

func (i *Interpreter) evalProgram(prog *ast.Program) Value {
	var result Value = NULL
	for _, stmt := range hoistDeclarations(prog.Statements) {
		result = i.Eval(stmt)
		if result != nil {
			if ret, ok := result.(*ReturnValue); ok {
				return ret.Val
			}
			if errVal, ok := result.(*ErrorValue); ok {
				return errVal
			}
		}
	}
	return result
}

func (i *Interpreter) evalVarDeclaration(node *ast.VarDeclaration) Value {
	var val Value = NULL
	if node.Value != nil {
		val = i.Eval(node.Value)
	}

	name := ""
	if node.Name != nil {
		name = node.Name.Value
	}

	// Validate type annotation if present
	if node.TypeAnnotation != nil {
		if !validateType(val, node.TypeAnnotation) {
			errVal := NewError(400, fmt.Sprintf("type mismatch for %s: expected %s, got %s", name, node.TypeAnnotation.String(), val.Type()))
			i.recordError(errVal)
			return errVal
		}
	}

	// Process variable decorators
	for _, d := range node.Decorators {
		if d == nil {
			continue
		}
		if d.Name == "drop_on_oom" {
			i.dropOnOOM[name] = true
		}
	}

	i.env.Set(name, val)
	i.trackMemory(int64(len(name) + 32))
	return val
}

func (i *Interpreter) evalFuncDeclaration(node *ast.FuncDeclaration) Value {
	fn := &FuncValue{
		Name:       node.Name,
		Params:     node.Params,
		ReturnType: node.ReturnType,
		Body:       node.Body,
		Env:        i.env,
		IsShell:    node.IsShell,
	}

	// Process decorators on function
	for _, d := range node.Decorators {
		if d == nil {
			continue
		}
		switch d.Name {
		case "error_log":
			if len(d.Args) > 0 {
				i.errorLog = CoerceToString(i.Eval(d.Args[0]))
			}
		case "oom_guard":
			for _, arg := range d.Args {
				argStr := CoerceToString(i.Eval(arg))
				if strings.HasPrefix(argStr, "max=") {
					i.oomMax = parseMemorySize(strings.TrimPrefix(argStr, "max="))
				} else if strings.HasPrefix(argStr, "policy=") {
					i.oomPolicy = strings.TrimPrefix(argStr, "policy=")
				}
			}
		}
	}

	i.env.Set(node.Name, fn)
	return fn
}

func (i *Interpreter) evalStructDeclaration(node *ast.StructDeclaration) Value {
	sDef := &StructDef{
		Name:   node.Name,
		Fields: node.Fields,
	}
	i.env.Set(node.Name, sDef)
	return sDef
}

func (i *Interpreter) evalIfStatement(node *ast.IfStatement) Value {
	cond := i.Eval(node.Condition)
	if IsTruthy(cond) {
		return i.Eval(node.Consequence)
	}

	for _, alt := range node.Alternatives {
		if alt == nil {
			continue
		}
		altCond := i.Eval(alt.Condition)
		if IsTruthy(altCond) {
			return i.Eval(alt.Body)
		}
	}

	if node.ElseBlock != nil {
		return i.Eval(node.ElseBlock)
	}

	return NULL
}

func (i *Interpreter) evalWhileStatement(node *ast.WhileStatement) Value {
	for {
		cond := i.Eval(node.Condition)
		if !IsTruthy(cond) {
			break
		}
		res := i.Eval(node.Body)
		if res != nil {
			if res.Type() == BREAK_VAL {
				break
			}
			if res.Type() == CONTINUE_VAL {
				continue
			}
			if res.Type() == RETURN_VAL || res.Type() == ERROR_VAL {
				return res
			}
		}
	}
	return NULL
}

func (i *Interpreter) evalForStatement(node *ast.ForStatement) Value {
	iter := i.Eval(node.Iterable)
	items := extractIterableItems(iter)

	for _, item := range items {
		i.env.Set(node.Variable, item)
		res := i.Eval(node.Body)
		if res != nil {
			if res.Type() == BREAK_VAL {
				break
			}
			if res.Type() == CONTINUE_VAL {
				continue
			}
			if res.Type() == RETURN_VAL || res.Type() == ERROR_VAL {
				return res
			}
		}
	}
	return NULL
}

func (i *Interpreter) evalForEachStatement(node *ast.ForEachStatement) Value {
	iter := i.Eval(node.Iterable)
	items := extractIterableItems(iter)

	for _, item := range items {
		i.env.Set(node.Variable, item)
		res := i.Eval(node.Body)
		if res != nil {
			if res.Type() == BREAK_VAL {
				break
			}
			if res.Type() == CONTINUE_VAL {
				continue
			}
			if res.Type() == RETURN_VAL || res.Type() == ERROR_VAL {
				return res
			}
		}
	}
	return NULL
}

func (i *Interpreter) evalLoopStatement(node *ast.LoopStatement) Value {
	for {
		res := i.Eval(node.Body)
		if res != nil {
			if res.Type() == BREAK_VAL {
				break
			}
			if res.Type() == CONTINUE_VAL {
				continue
			}
			if res.Type() == RETURN_VAL || res.Type() == ERROR_VAL {
				return res
			}
		}
	}
	return NULL
}

func (i *Interpreter) evalBlockStatement(node *ast.BlockStatement) Value {
	var result Value = NULL
	for _, stmt := range hoistDeclarations(node.Statements) {
		result = i.Eval(stmt)
		if result != nil {
			rt := result.Type()
			if rt == RETURN_VAL || rt == BREAK_VAL || rt == CONTINUE_VAL || rt == ERROR_VAL {
				return result
			}
		}
	}
	return result
}

func (i *Interpreter) evalUseStatement(node *ast.UseStatement) Value {
	modPath := node.ModulePath

	// Handle built-in standard library modules
	if strings.HasPrefix(modPath, "std.") {
		return i.loadStdModule(node)
	}

	// If module is already registered in environment or globals as a map
	if registered, ok := i.env.Get(modPath); ok {
		if _, isMap := registered.(*MapValue); isMap {
			return i.loadStdModule(node)
		}
	}
	if registered, ok := i.globals.Get(modPath); ok {
		if _, isMap := registered.(*MapValue); isMap {
			return i.loadStdModule(node)
		}
	}

	// Handle local/relative modules (e.g. models.user -> models/user.al)
	relPath := strings.ReplaceAll(modPath, ".", string(filepath.Separator)) + ".al"
	content, err := ReadFile(relPath)
	if err != nil {
		errVal := NewError(404, fmt.Sprintf("cannot load module %q: %s", modPath, err.Error()))
		i.recordError(errVal)
		return errVal
	}

	moduleParserMu.RLock()
	parser := activeModuleParser
	moduleParserMu.RUnlock()

	if parser == nil {
		return NULL
	}

	prog, err := parser(content)
	if err != nil {
		errVal := NewError(500, fmt.Sprintf("module %q parse error: %s", modPath, err.Error()))
		i.recordError(errVal)
		return errVal
	}

	modEnv := NewEnvironment()
	for name, fn := range BuiltinFunctionMap {
		modEnv.Set(name, fn)
	}

	subInterp := i.Fork(modEnv)
	if err := subInterp.Execute(prog); err != nil {
		errVal := NewError(500, err.Error())
		i.recordError(errVal)
		return errVal
	}

	// Selective imports or alias binding
	if len(node.Imports) > 0 {
		for _, sym := range node.Imports {
			if val, ok := modEnv.Get(sym); ok {
				i.env.Set(sym, val)
			}
		}
	} else if node.Alias != "" {
		aliasMap := NewMap()
		for k, v := range modEnv.All() {
			aliasMap.Set(k, v)
		}
		i.env.Set(node.Alias, aliasMap)
	} else {
		for k, v := range modEnv.All() {
			i.env.Set(k, v)
		}
	}

	return NULL
}

func (i *Interpreter) loadStdModule(node *ast.UseStatement) Value {
	modMap := NewMap()
	shortName := strings.TrimPrefix(node.ModulePath, "std.")

	// Check if module is already registered in environment or globals (e.g. math, io, sys, cli, etc.)
	if registered, ok := i.env.Get(node.ModulePath); ok {
		if m, isMap := registered.(*MapValue); isMap {
			modMap = m
		}
	} else if registered, ok := i.globals.Get(node.ModulePath); ok {
		if m, isMap := registered.(*MapValue); isMap {
			modMap = m
		}
	} else if registered, ok := i.env.Get(shortName); ok {
		if m, isMap := registered.(*MapValue); isMap {
			modMap = m
		}
	} else if registered, ok := i.globals.Get(shortName); ok {
		if m, isMap := registered.(*MapValue); isMap {
			modMap = m
		}
	} else {
		switch node.ModulePath {
		case "std.formats":
			modMap.Set("json_encode", BuiltinFunctionMap["json_encode"])
			modMap.Set("json_decode", BuiltinFunctionMap["json_decode"])
		case "std.db.postgres":
			modMap.Set("connect", &BuiltinFunction{
				Name: "connect",
				Fn: func(args ...Value) Value {
					return &StringValue{Val: "postgres://mock_connection"}
				},
			})
		case "std.math":
			modMap.Set("abs", BuiltinFunctionMap["abs"])
			modMap.Set("max", BuiltinFunctionMap["max"])
			modMap.Set("min", BuiltinFunctionMap["min"])
		}
	}

	if len(node.Imports) > 0 {
		for _, imp := range node.Imports {
			if val, ok := modMap.Get(imp); ok {
				i.env.Set(imp, val)
			}
		}
	} else if node.Alias != "" {
		i.env.Set(node.Alias, modMap)
	} else {
		i.env.Set(shortName, modMap)
		// Flatten the module members as convenience globals, but never shadow an
		// existing binding: module names (e.g. math.log vs the std.log module),
		// global builtins and user variables must keep their meaning.
		for _, k := range modMap.Keys() {
			if _, exists := i.env.Get(k); exists {
				continue
			}
			val, _ := modMap.Get(k)
			i.env.Set(k, val)
		}
	}

	return NULL
}

func (i *Interpreter) evalWatchStatement(node *ast.WatchStatement) Value {
	if node.Target == nil || node.Body == nil {
		return NULL
	}

	targetPath := CoerceToString(i.Eval(node.Target))
	AddAsync()
	go func() {
		defer DoneAsync()
		var lastMod time.Time
		if info, err := os.Stat(targetPath); err == nil {
			lastMod = info.ModTime()
		}

		for {
			time.Sleep(500 * time.Millisecond)
			info, err := os.Stat(targetPath)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				childInterp := i.Fork(NewEnclosedEnvironment(i.env))
				_ = childInterp.Eval(node.Body)
			}
		}
	}()
	return NULL
}

func (i *Interpreter) evalTestStatement(node *ast.TestStatement) Value {
	testEnv := NewEnclosedEnvironment(i.env)
	childInterp := i.Fork(testEnv)

	res := childInterp.Eval(node.Body)
	if errVal, ok := res.(*ErrorValue); ok {
		fmt.Printf("[FAIL] Test: %s - %s\n", node.Name, errVal.Message)
		return errVal
	}
	fmt.Printf("[PASS] Test: %s\n", node.Name)
	return TRUE
}

func (i *Interpreter) evalMockStatement(node *ast.MockStatement) Value {
	targetName := ""
	if id, ok := node.Target.(*ast.Identifier); ok {
		targetName = id.Value
	} else {
		targetName = CoerceToString(i.Eval(node.Target))
	}

	replacement := i.Eval(node.Replacement)
	i.mu.Lock()
	i.mocks[targetName] = replacement
	i.mu.Unlock()
	return replacement
}

func (i *Interpreter) evalBaStatement(node *ast.BaStatement) Value {
	if node.Body == nil {
		return NULL
	}

	AddAsync()
	go func() {
		defer DoneAsync()
		childInterp := i.Fork(NewEnclosedEnvironment(i.env))

		if node.Timeout != nil {
			timeoutDur := i.parseDuration(i.Eval(node.Timeout))
			if timeoutDur > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), timeoutDur)
				defer cancel()
				done := make(chan struct{})
				go func() {
					_ = childInterp.Eval(node.Body)
					close(done)
				}()
				select {
				case <-done:
				case <-ctx.Done():
					childInterp.recordError(NewError(408, "background task timed out"))
				}
				return
			}
		}
		_ = childInterp.Eval(node.Body)
	}()
	return NULL
}

func (i *Interpreter) evalBaForStatement(node *ast.BaForStatement) Value {
	items := extractIterableItems(i.Eval(node.Iterable))
	limit := 10
	if node.Limit != nil {
		if iv, ok := i.Eval(node.Limit).(*IntegerValue); ok && iv.Val > 0 {
			limit = int(iv.Val)
		}
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(val Value) {
			defer func() {
				<-sem
				wg.Done()
			}()
			childEnv := NewEnclosedEnvironment(i.env)
			childEnv.Set(node.Variable, val)
			childInterp := i.Fork(childEnv)
			_ = childInterp.Eval(node.Body)
		}(item)
	}
	wg.Wait()
	return NULL
}

func (i *Interpreter) evalThreadsStatement(node *ast.ThreadsStatement) Value {
	if node.Body == nil {
		return NULL
	}
	var wg sync.WaitGroup
	for _, stmt := range node.Body.Statements {
		if stmt == nil {
			continue
		}
		wg.Add(1)
		go func(s ast.Statement) {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()
			childInterp := i.Fork(NewEnclosedEnvironment(i.env))
			_ = childInterp.Eval(s)
		}(stmt)
	}
	wg.Wait()
	return NULL
}

func (i *Interpreter) evalMatchStatement(node *ast.MatchStatement) Value {
	subject := i.Eval(node.Subject)

	for _, mc := range node.Cases {
		if mc == nil {
			continue
		}
		casePat := i.Eval(mc.Pattern)
		if ValuesEqual(subject, casePat) {
			return i.Eval(mc.Body)
		}
	}

	if node.Default != nil {
		return i.Eval(node.Default)
	}
	return NULL
}

func (i *Interpreter) evalTxStatement(node *ast.TxStatement) Value {
	txEnv := NewEnclosedEnvironment(i.env)
	childInterp := i.Fork(txEnv)

	res := childInterp.Eval(node.Body)
	if errVal, ok := res.(*ErrorValue); ok {
		return errVal
	}
	// Commit updates back to caller environment
	for k, v := range txEnv.All() {
		i.env.Set(k, v)
	}
	return res
}

// -----------------------------------------------------------------------------
// Expression Evaluators
// -----------------------------------------------------------------------------

func (i *Interpreter) evalIdentifier(node *ast.Identifier) Value {
	i.mu.RLock()
	mock, hasMock := i.mocks[node.Value]
	i.mu.RUnlock()
	if hasMock {
		return mock
	}

	if val, ok := i.env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := BuiltinFunctionMap[node.Value]; ok {
		return builtin
	}
	return NewError(404, fmt.Sprintf("undefined identifier: %s", node.Value))
}

func (i *Interpreter) evalPrefixExpression(node *ast.PrefixExpression) Value {
	right := i.Eval(node.Right)
	if errVal, ok := right.(*ErrorValue); ok {
		return errVal
	}

	switch node.Operator {
	case "-":
		switch val := right.(type) {
		case *IntegerValue:
			return &IntegerValue{Val: -val.Val}
		case *FloatValue:
			return &FloatValue{Val: -val.Val}
		default:
			return NewError(400, fmt.Sprintf("unsupported operand type for -: %s", right.Type()))
		}
	case "not", "!":
		if IsTruthy(right) {
			return FALSE
		}
		return TRUE
	default:
		return NewError(400, fmt.Sprintf("unknown prefix operator %s", node.Operator))
	}
}

func (i *Interpreter) evalInfixExpression(node *ast.InfixExpression) Value {
	// Handle short-circuit logical operators
	if node.Operator == "and" {
		left := i.Eval(node.Left)
		if !IsTruthy(left) {
			return left
		}
		return i.Eval(node.Right)
	}
	if node.Operator == "or" {
		left := i.Eval(node.Left)
		if IsTruthy(left) {
			return left
		}
		return i.Eval(node.Right)
	}

	left := i.Eval(node.Left)
	right := i.Eval(node.Right)

	if errVal, ok := left.(*ErrorValue); ok {
		return errVal
	}
	if errVal, ok := right.(*ErrorValue); ok {
		return errVal
	}

	switch node.Operator {
	case "==":
		if ValuesEqual(left, right) {
			return TRUE
		}
		return FALSE

	case "!=":
		if !ValuesEqual(left, right) {
			return TRUE
		}
		return FALSE

	case "+":
		// String concatenation
		if left.Type() == STRING_VAL || right.Type() == STRING_VAL {
			return &StringValue{Val: CoerceToString(left) + CoerceToString(right)}
		}
		// List concatenation
		if left.Type() == LIST_VAL && right.Type() == LIST_VAL {
			l1 := left.(*ListValue)
			l2 := right.(*ListValue)
			combined := make([]Value, 0, len(l1.Elements)+len(l2.Elements))
			combined = append(combined, l1.Elements...)
			combined = append(combined, l2.Elements...)
			return NewList(combined...)
		}
		// Numeric addition
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			return &IntegerValue{Val: left.(*IntegerValue).Val + right.(*IntegerValue).Val}
		}
		return &FloatValue{Val: getNumericFloat(left) + getNumericFloat(right)}

	case "-":
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			return &IntegerValue{Val: left.(*IntegerValue).Val - right.(*IntegerValue).Val}
		}
		return &FloatValue{Val: getNumericFloat(left) - getNumericFloat(right)}

	case "*":
		// String repetition
		if left.Type() == STRING_VAL && right.Type() == INTEGER_VAL {
			return &StringValue{Val: strings.Repeat(left.(*StringValue).Val, int(right.(*IntegerValue).Val))}
		}
		// List repetition
		if left.Type() == LIST_VAL && right.Type() == INTEGER_VAL {
			l := left.(*ListValue)
			times := int(right.(*IntegerValue).Val)
			repeated := make([]Value, 0, len(l.Elements)*times)
			for idx := 0; idx < times; idx++ {
				repeated = append(repeated, l.Elements...)
			}
			return NewList(repeated...)
		}
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			return &IntegerValue{Val: left.(*IntegerValue).Val * right.(*IntegerValue).Val}
		}
		return &FloatValue{Val: getNumericFloat(left) * getNumericFloat(right)}

	case "/":
		// Typed semantics: int / int is truncating integer division, otherwise float division.
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			i1 := left.(*IntegerValue).Val
			i2 := right.(*IntegerValue).Val
			if i2 == 0 {
				return NewError(400, "division by zero")
			}
			return &IntegerValue{Val: i1 / i2}
		}
		f2 := getNumericFloat(right)
		if f2 == 0 {
			return NewError(400, "division by zero")
		}
		return &FloatValue{Val: getNumericFloat(left) / f2}

	case "%":
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			i2 := right.(*IntegerValue).Val
			if i2 == 0 {
				return NewError(400, "modulo by zero")
			}
			return &IntegerValue{Val: left.(*IntegerValue).Val % i2}
		}
		return NewError(400, "modulo requires integer operands")

	case "**":
		if left.Type() == INTEGER_VAL && right.Type() == INTEGER_VAL {
			exp := right.(*IntegerValue).Val
			if exp < 0 {
				return NewError(400, "negative exponent for integer power; use float operands")
			}
			return &IntegerValue{Val: IntPow(left.(*IntegerValue).Val, exp)}
		}
		return &FloatValue{Val: math.Pow(getNumericFloat(left), getNumericFloat(right))}

	case "in":
		return InOp(left, right)

	case "<":
		if left.Type() == STRING_VAL && right.Type() == STRING_VAL {
			return &BoolValue{Val: left.(*StringValue).Val < right.(*StringValue).Val}
		}
		return &BoolValue{Val: getNumericFloat(left) < getNumericFloat(right)}

	case ">":
		if left.Type() == STRING_VAL && right.Type() == STRING_VAL {
			return &BoolValue{Val: left.(*StringValue).Val > right.(*StringValue).Val}
		}
		return &BoolValue{Val: getNumericFloat(left) > getNumericFloat(right)}

	case "<=":
		if left.Type() == STRING_VAL && right.Type() == STRING_VAL {
			return &BoolValue{Val: left.(*StringValue).Val <= right.(*StringValue).Val}
		}
		return &BoolValue{Val: getNumericFloat(left) <= getNumericFloat(right)}

	case ">=":
		if left.Type() == STRING_VAL && right.Type() == STRING_VAL {
			return &BoolValue{Val: left.(*StringValue).Val >= right.(*StringValue).Val}
		}
		return &BoolValue{Val: getNumericFloat(left) >= getNumericFloat(right)}

	default:
		return NewError(400, fmt.Sprintf("unknown infix operator %s", node.Operator))
	}
}

// InOp evaluates the `left in right` membership operator. Both engines share
// this helper so list/map/string membership can never diverge:
//
//	list   -> true when any element equals left
//	map    -> true when left is one of the keys
//	string -> true when left is a substring of right
func InOp(left, right Value) Value {
	switch coll := right.(type) {
	case *ListValue:
		for _, el := range coll.Elements {
			if ValuesEqual(left, el) {
				return TRUE
			}
		}
		return FALSE
	case *MapValue:
		if coll.Has(CoerceToString(left)) {
			return TRUE
		}
		return FALSE
	case *StringValue:
		if strings.Contains(coll.Val, CoerceToString(left)) {
			return TRUE
		}
		return FALSE
	default:
		return NewError(400, "'in' needs a list, map or string on the right")
	}
}

func (i *Interpreter) evalCallExpression(node *ast.CallExpression) Value {
	// Handle method calls on objects: obj.method(args...)
	if dot, ok := node.Function.(*ast.DotExpression); ok {
		target := i.Eval(dot.Left)
		if errVal, ok := target.(*ErrorValue); ok {
			return errVal
		}
		args := make([]Value, len(node.Arguments))
		for idx, arg := range node.Arguments {
			args[idx] = i.Eval(arg)
			if ev, isErr := args[idx].(*ErrorValue); isErr {
				return ev
			}
		}
		res, err := CallMethod(target, dot.Field, args)
		if err == nil {
			return res
		}
		// If CallMethod didn't find a built-in method, check struct instance fields
		if inst, ok := target.(*StructInstance); ok {
			if fVal, exists := inst.Fields[dot.Field]; exists {
				ret, invErr := i.invokeFunction(fVal, args, node.KwArgs)
				if invErr != nil {
					return NewError(500, invErr.Error())
				}
				return ret
			}
		}
		// Check map entries for callable values (modules, UI objects, etc.)
		if m, ok := target.(*MapValue); ok {
			if fVal, exists := m.Get(dot.Field); exists {
				ret, invErr := i.invokeFunction(fVal, args, node.KwArgs)
				if invErr != nil {
					return NewError(500, invErr.Error())
				}
				return ret
			}
		}
		return NewError(400, err.Error())
	}

	fnVal := i.Eval(node.Function)
	if errVal, ok := fnVal.(*ErrorValue); ok {
		return errVal
	}

	args := make([]Value, len(node.Arguments))
	for idx, arg := range node.Arguments {
		args[idx] = i.Eval(arg)
		if ev, isErr := args[idx].(*ErrorValue); isErr {
			return ev // an error in an argument aborts the call
		}
	}

	res, err := i.invokeFunction(fnVal, args, node.KwArgs)
	if err != nil {
		return NewError(500, err.Error())
	}
	return res
}

// InvokeFunction invokes a function value (BuiltinFunction, StructDef, FuncValue) with arguments.
func (i *Interpreter) InvokeFunction(fnVal Value, args []Value) (Value, error) {
	return i.invokeFunction(fnVal, args, nil)
}

func (i *Interpreter) invokeFunction(fnVal Value, args []Value, kwArgs map[string]ast.Expression) (Value, error) {
	switch fn := fnVal.(type) {
	case *BuiltinFunction:
		return fn.Fn(args...), nil

	case *StructDef:
		return i.instantiateStruct(fn, args, kwArgs), nil

	case *FuncValue:
		// Check for test mock override
		i.mu.RLock()
		mock, hasMock := i.mocks[fn.Name]
		i.mu.RUnlock()
		if hasMock {
			return i.invokeFunction(mock, args, kwArgs)
		}

		if fn.IsShell {
			return i.executeShellFunc(fn, args), nil
		}

		callEnv := NewEnclosedEnvironment(fn.Env)
		// Bind positional parameters
		for idx, param := range fn.Params {
			if idx < len(args) {
				callEnv.Set(param.Name, args[idx])
			} else if param.DefaultValue != nil {
				callEnv.Set(param.Name, i.Eval(param.DefaultValue))
			} else {
				callEnv.Set(param.Name, NULL)
			}
		}
		// Bind keyword arguments
		for k, kwExpr := range kwArgs {
			callEnv.Set(k, i.Eval(kwExpr))
		}

		oldEnv := i.env
		i.env = callEnv
		res := i.Eval(fn.Body)
		i.env = oldEnv

		if ret, ok := res.(*ReturnValue); ok {
			res = ret.Val
		}

		// Validate return type annotation if declared
		if _, isErr := res.(*ErrorValue); !isErr && fn.ReturnType != nil && !validateType(res, fn.ReturnType) {
			return NewError(400, fmt.Sprintf("invalid return type: expected %s, got %s", fn.ReturnType.String(), res.Type())), nil
		}
		return res, nil

	default:
		return nil, fmt.Errorf("value of type %s is not callable", fnVal.Type())
	}
}

func (i *Interpreter) executeShellFunc(fn *FuncValue, args []Value) Value {
	var capturedCmds []string
	prevHook := activePrintHook

	SetPrintHook(func(out string) {
		capturedCmds = append(capturedCmds, out)
	})
	defer func() {
		if prevHook != nil {
			SetPrintHook(prevHook)
		} else {
			ResetPrintHook()
		}
	}()

	callEnv := NewEnclosedEnvironment(fn.Env)
	for idx, param := range fn.Params {
		if idx < len(args) {
			callEnv.Set(param.Name, args[idx])
		} else if param.DefaultValue != nil {
			callEnv.Set(param.Name, i.Eval(param.DefaultValue))
		}
	}

	oldEnv := i.env
	i.env = callEnv
	_ = i.Eval(fn.Body)
	i.env = oldEnv

	outputs := make([]string, 0, len(capturedCmds))
	for _, cmd := range capturedCmds {
		clean := strings.TrimSpace(cmd)
		if clean == "" {
			continue
		}
		out, _ := RunShellCommand(clean)
		outputs = append(outputs, strings.TrimRight(out, "\r\n"))
	}
	return &StringValue{Val: strings.Join(outputs, "\n")}
}

func (i *Interpreter) instantiateStruct(sDef *StructDef, args []Value, kwArgs map[string]ast.Expression) Value {
	fields := make(map[string]Value, len(sDef.Fields))

	// Initialize default values
	for _, f := range sDef.Fields {
		if f.DefaultValue != nil {
			fields[f.Name] = i.Eval(f.DefaultValue)
		} else {
			fields[f.Name] = NULL
		}
	}

	// Populate positional arguments in order
	for idx, a := range args {
		if idx < len(sDef.Fields) {
			fields[sDef.Fields[idx].Name] = a
		}
	}

	// Populate keyword arguments
	for k, kwExpr := range kwArgs {
		fields[k] = i.Eval(kwExpr)
	}

	return &StructInstance{
		Def:    sDef,
		Fields: fields,
	}
}

func (i *Interpreter) evalIndexExpression(node *ast.IndexExpression) Value {
	left := i.Eval(node.Left)
	index := i.Eval(node.Index)

	switch target := left.(type) {
	case *ListValue:
		iv, ok := index.(*IntegerValue)
		if !ok {
			return NewError(400, "list index must be an integer")
		}
		idx := iv.Val
		n := int64(len(target.Elements))
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return NewError(400, fmt.Sprintf("index %d out of bounds (len %d)", iv.Val, n))
		}
		return target.Elements[idx]

	case *MapValue:
		key := CoerceToString(index)
		val, ok := target.Get(key)
		if !ok {
			return NULL
		}
		return val

	case *StringValue:
		iv, ok := index.(*IntegerValue)
		if !ok {
			return NewError(400, "string index must be an integer")
		}
		runes := []rune(target.Val)
		idx := iv.Val
		n := int64(len(runes))
		if idx < 0 {
			idx += n
		}
		if idx < 0 || idx >= n {
			return NewError(400, fmt.Sprintf("string index %d out of bounds", iv.Val))
		}
		return &StringValue{Val: string(runes[idx])}

	default:
		return NewError(400, fmt.Sprintf("cannot index value of type %s", left.Type()))
	}
}

func (i *Interpreter) evalDotExpression(node *ast.DotExpression) Value {
	left := i.Eval(node.Left)

	switch target := left.(type) {
	case *StructInstance:
		if val, exists := target.Fields[node.Field]; exists {
			return val
		}
		return NULL

	case *MapValue:
		if val, exists := target.Get(node.Field); exists {
			return val
		}
		return NULL

	case *ErrorValue:
		switch node.Field {
		case "msg", "message":
			return &StringValue{Val: target.Message}
		case "code":
			return &IntegerValue{Val: target.Code}
		case "time":
			return &StringValue{Val: target.Time}
		default:
			return NULL
		}

	default:
		// Attempt standard method dispatch with zero arguments
		res, err := CallMethod(left, node.Field, nil)
		if err == nil {
			return res
		}
		return NewError(400, fmt.Sprintf("field %q not found on type %s", node.Field, left.Type()))
	}
}

func (i *Interpreter) evalAssignExpression(node *ast.AssignExpression) Value {
	val := i.Eval(node.Value)

	switch target := node.Target.(type) {
	case *ast.Identifier:
		// Instant memory release if null is assigned to an existing variable
		if val.Type() == NULL_VAL {
			i.env.Delete(target.Value)
			return NULL
		}
		if _, updated := i.env.Update(target.Value, val); !updated {
			i.env.Set(target.Value, val)
		}
		return val

	case *ast.DotExpression:
		left := i.Eval(target.Left)
		if inst, ok := left.(*StructInstance); ok {
			inst.Fields[target.Field] = val
			return val
		}
		if m, ok := left.(*MapValue); ok {
			m.Set(target.Field, val)
			return val
		}
		return NewError(400, "cannot assign to dot target")

	case *ast.IndexExpression:
		left := i.Eval(target.Left)
		idx := i.Eval(target.Index)

		if l, ok := left.(*ListValue); ok {
			if iv, ok := idx.(*IntegerValue); ok {
				iVal := iv.Val
				n := int64(len(l.Elements))
				if iVal < 0 {
					iVal += n
				}
				if iVal >= 0 && iVal < n {
					l.Elements[iVal] = val
					return val
				}
			}
			return NewError(400, "invalid list index for assignment")
		}
		if m, ok := left.(*MapValue); ok {
			m.Set(CoerceToString(idx), val)
			return val
		}
		return NewError(400, "cannot assign to indexed target")

	default:
		return NewError(400, "invalid assignment target")
	}
}

func (i *Interpreter) evalPipeExpression(node *ast.PipeExpression) Value {
	leftVal := i.Eval(node.Left)

	// If right expression is a CallExpression, prepend leftVal as first argument
	if call, ok := node.Right.(*ast.CallExpression); ok {
		evalArgs := make([]Value, 0, len(call.Arguments)+1)
		evalArgs = append(evalArgs, leftVal)
		for _, arg := range call.Arguments {
			evalArgs = append(evalArgs, i.Eval(arg))
		}
		fnVal := i.Eval(call.Function)
		res, err := i.invokeFunction(fnVal, evalArgs, call.KwArgs)
		if err != nil {
			return NewError(500, err.Error())
		}
		return res
	}

	rightVal := i.Eval(node.Right)
	res, err := i.invokeFunction(rightVal, []Value{leftVal}, nil)
	if err != nil {
		return NewError(500, err.Error())
	}
	return res
}

func (i *Interpreter) evalTimeoutExpression(node *ast.TimeoutExpression) Value {
	dur := i.parseDuration(i.Eval(node.Duration))
	if dur <= 0 {
		return i.Eval(node.Expression)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	resultChan := make(chan Value, 1)
	go func() {
		resultChan <- i.Eval(node.Expression)
	}()

	select {
	case res := <-resultChan:
		return res
	case <-ctx.Done():
		return NewError(408, fmt.Sprintf("execution timed out after %s", dur.String()))
	}
}

// -----------------------------------------------------------------------------
// Helper Utilities
// -----------------------------------------------------------------------------

func extractIterableItems(v Value) []Value {
	switch val := v.(type) {
	case *ListValue:
		return val.Elements
	case *MapValue:
		keys := val.Keys()
		items := make([]Value, len(keys))
		for idx, k := range keys {
			items[idx] = &StringValue{Val: k}
		}
		return items
	case *StringValue:
		runes := []rune(val.Val)
		items := make([]Value, len(runes))
		for idx, r := range runes {
			items[idx] = &StringValue{Val: string(r)}
		}
		return items
	case *IntegerValue:
		// 'for i in n' iterates 0..n-1 (see the type checker's forLoop).
		n := val.Val
		if n < 0 {
			n = 0
		}
		items := make([]Value, n)
		for idx := int64(0); idx < n; idx++ {
			items[idx] = &IntegerValue{Val: idx}
		}
		return items
	default:
		return []Value{v}
	}
}

// EvalRange generates an integer sequence list from start to end inclusive.
func EvalRange(start, end Value) Value {
	sInt, ok1 := start.(*IntegerValue)
	eInt, ok2 := end.(*IntegerValue)
	if !ok1 || !ok2 {
		return NewList()
	}

	step := int64(1)
	if sInt.Val > eInt.Val {
		step = -1
	}

	elems := make([]Value, 0)
	if step > 0 {
		for v := sInt.Val; v <= eInt.Val; v += step {
			elems = append(elems, &IntegerValue{Val: v})
		}
	} else {
		for v := sInt.Val; v >= eInt.Val; v += step {
			elems = append(elems, &IntegerValue{Val: v})
		}
	}
	return NewList(elems...)
}

func evalRange(start, end Value) Value {
	return EvalRange(start, end)
}

func validateType(val Value, ta *ast.TypeAnnotation) bool {
	if ta == nil || len(ta.Types) == 0 {
		return true
	}
	if val == nil {
		val = NULL
	}

	for _, expected := range ta.Types {
		expected = strings.TrimSpace(expected)
		// Generic parameters (list[int], map[string, int]) are verified statically by
		// the type checker; at runtime only the container kind is checked.
		if idx := strings.IndexByte(expected, '['); idx > 0 {
			expected = expected[:idx]
		}
		switch expected {
		case "error":
			if val.Type() == ERROR_VAL {
				return true
			}
		case "int":
			if val.Type() == INTEGER_VAL {
				return true
			}
		case "float":
			if val.Type() == FLOAT_VAL {
				return true
			}
		case "string":
			if val.Type() == STRING_VAL {
				return true
			}
		case "bool":
			if val.Type() == BOOL_VAL {
				return true
			}
		case "list":
			if val.Type() == LIST_VAL {
				return true
			}
		case "map":
			if val.Type() == MAP_VAL {
				return true
			}
		case "blob":
			if val.Type() == BLOB_VAL {
				return true
			}
		case "secret":
			if val.Type() == SECRET_VAL {
				return true
			}
		case "null":
			if val.Type() == NULL_VAL {
				return true
			}
		case "json":
			return true // dynamic type: any value
		default:
			// Check user-defined struct type name
			if inst, ok := val.(*StructInstance); ok && inst.Def.Name == expected {
				return true
			}
		}
	}
	return false
}

func (i *Interpreter) parseDuration(v Value) time.Duration {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case *IntegerValue:
		return time.Duration(val.Val) * time.Millisecond
	case *FloatValue:
		return time.Duration(val.Val * float64(time.Second))
	case *StringValue:
		dur, err := time.ParseDuration(val.Val)
		if err == nil {
			return dur
		}
		return 0
	default:
		return 0
	}
}

func parseMemorySize(raw string) int64 {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	multiplier := int64(1)
	if strings.HasSuffix(raw, "GB") {
		multiplier = 1024 * 1024 * 1024
		raw = strings.TrimSuffix(raw, "GB")
	} else if strings.HasSuffix(raw, "MB") {
		multiplier = 1024 * 1024
		raw = strings.TrimSuffix(raw, "MB")
	} else if strings.HasSuffix(raw, "KB") {
		multiplier = 1024
		raw = strings.TrimSuffix(raw, "KB")
	} else if strings.HasSuffix(raw, "B") {
		raw = strings.TrimSuffix(raw, "B")
	}

	num, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return num * multiplier
}

// hoistDeclarations returns stmts with function/struct declarations first (stable order),
// so they can be used before their textual definition (the type checker allows this).
func hoistDeclarations(stmts []ast.Statement) []ast.Statement {
	needs := false
	seenOther := false
	for _, st := range stmts {
		switch st.(type) {
		case *ast.FuncDeclaration, *ast.StructDeclaration:
			if seenOther {
				needs = true
			}
		default:
			seenOther = true
		}
	}
	if !needs {
		return stmts
	}
	out := make([]ast.Statement, 0, len(stmts))
	for _, st := range stmts {
		switch st.(type) {
		case *ast.FuncDeclaration, *ast.StructDeclaration:
			out = append(out, st)
		}
	}
	for _, st := range stmts {
		switch st.(type) {
		case *ast.FuncDeclaration, *ast.StructDeclaration:
		default:
			out = append(out, st)
		}
	}
	return out
}
