package ast

// Pos represents a source position (line and column).
type Pos struct {
	Line   int
	Column int
}

// Node is the interface that all AST nodes implement.
type Node interface {
	nodeType() string
	GetPos() Pos
}

// Stmt is the interface for statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the interface for expression nodes.
type Expr interface {
	Node
	exprNode()
}

// ============================================================
// Program (top-level)
// ============================================================

// Program represents the top-level AST node containing all statements.
type Program struct {
	Stmts []Stmt
}

func (p *Program) nodeType() string { return "Program" }
func (p *Program) GetPos() Pos {
	if len(p.Stmts) > 0 {
		return p.Stmts[0].GetPos()
	}
	return Pos{Line: 1, Column: 1}
}

// ============================================================
// Statements
// ============================================================

// AssignStmt represents assignment: a, b = 1, 2
type AssignStmt struct {
	P       Pos
	Targets []Expr
	Values  []Expr
}

func (s *AssignStmt) nodeType() string { return "AssignStmt" }
func (s *AssignStmt) GetPos() Pos      { return s.P }
func (s *AssignStmt) stmtNode()        {}

// DeclareStmt represents variable declaration: a, b := 1, 2; const a = 1
type DeclareStmt struct {
	P        Pos
	Names    []string
	Values   []Expr
	ReadOnly bool
}

func (s *DeclareStmt) nodeType() string { return "DeclareStmt" }
func (s *DeclareStmt) GetPos() Pos      { return s.P }
func (s *DeclareStmt) stmtNode()        {}

// CompoundAssignStmt represents compound assignment: a += b, a -= b, etc.
type CompoundAssignStmt struct {
	P      Pos
	Target Expr
	Op     string // "+=", "-=", "*=", "/="
	Value  Expr
}

func (s *CompoundAssignStmt) nodeType() string { return "CompoundAssignStmt" }
func (s *CompoundAssignStmt) GetPos() Pos      { return s.P }
func (s *CompoundAssignStmt) stmtNode()        {}

// IncDecStmt represents increment/decrement: a++, a--
type IncDecStmt struct {
	P      Pos
	Target Expr
	Op     string // "++" or "--"
}

func (s *IncDecStmt) nodeType() string { return "IncDecStmt" }
func (s *IncDecStmt) GetPos() Pos      { return s.P }
func (s *IncDecStmt) stmtNode()        {}

// CallStmt wraps a CallExpr as a statement.
type CallStmt struct {
	P    Pos
	Call *CallExpr
}

func (s *CallStmt) nodeType() string { return "CallStmt" }
func (s *CallStmt) GetPos() Pos      { return s.P }
func (s *CallStmt) stmtNode()        {}

// GoStmt represents a go statement: go func(){}() or go f(args)
type GoStmt struct {
	P    Pos
	Call Expr // must be *CallExpr or *MethodCallExpr
}

func (s *GoStmt) nodeType() string { return "GoStmt" }
func (s *GoStmt) GetPos() Pos      { return s.P }
func (s *GoStmt) stmtNode()        {}

// DeferStmt represents a deferred call: defer f(args).
type DeferStmt struct {
	P    Pos
	Call Expr // must be *CallExpr or *MethodCallExpr
}

func (s *DeferStmt) nodeType() string { return "DeferStmt" }
func (s *DeferStmt) GetPos() Pos      { return s.P }
func (s *DeferStmt) stmtNode()        {}

// SendStmt represents a channel send: ch <- value
type SendStmt struct {
	P       Pos
	Channel Expr
	Value   Expr
}

func (s *SendStmt) nodeType() string { return "SendStmt" }
func (s *SendStmt) GetPos() Pos      { return s.P }
func (s *SendStmt) stmtNode()        {}

// SelectStmt represents a Go-style non-blocking select statement.
type SelectStmt struct {
	P       Pos
	Cases   []SelectCase
	Default *BlockStmt
}

// SelectCase represents either a receive or send case inside select.
type SelectCase struct {
	P          Pos
	RecvName   string // optional for: case name := <-ch:
	RecvOkName string // optional for: case name, ok := <-ch:
	Channel    Expr
	SendValue  Expr // nil means receive case; non-nil means send case
	Body       *BlockStmt
}

func (s *SelectStmt) nodeType() string { return "SelectStmt" }
func (s *SelectStmt) GetPos() Pos      { return s.P }
func (s *SelectStmt) stmtNode()        {}

// IfStmt represents if/elseif/else chains.
type IfStmt struct {
	P        Pos
	Cond     Expr
	Body     *BlockStmt
	ElseIfs  []ElseIfClause
	ElseBody *BlockStmt // nil if no else
}

// ElseIfClause represents a single elseif branch.
type ElseIfClause struct {
	P    Pos
	Cond Expr
	Body *BlockStmt
}

func (s *IfStmt) nodeType() string { return "IfStmt" }
func (s *IfStmt) GetPos() Pos      { return s.P }
func (s *IfStmt) stmtNode()        {}

// ForNumStmt represents a C-style for loop: for i := 0; i < n; i++ { }
type ForNumStmt struct {
	P    Pos
	Init Stmt // the init statement (typically DeclareStmt or AssignStmt)
	Cond Expr
	Post Stmt // the post statement (typically IncDecStmt or CompoundAssignStmt)
	Body *BlockStmt
}

func (s *ForNumStmt) nodeType() string { return "ForNumStmt" }
func (s *ForNumStmt) GetPos() Pos      { return s.P }
func (s *ForNumStmt) stmtNode()        {}

// ForRangeStmt represents a range-based for loop: for k, v := range expr { }
type ForRangeStmt struct {
	P     Pos
	Key   string // first variable name
	Value string // second variable name (may be empty)
	Iter  Expr   // the expression being iterated
	Body  *BlockStmt
}

func (s *ForRangeStmt) nodeType() string { return "ForRangeStmt" }
func (s *ForRangeStmt) GetPos() Pos      { return s.P }
func (s *ForRangeStmt) stmtNode()        {}

// ForStmt represents a while-style loop: for cond { }
type ForStmt struct {
	P    Pos
	Cond Expr // nil means infinite loop (for { })
	Body *BlockStmt
}

func (s *ForStmt) nodeType() string { return "ForStmt" }
func (s *ForStmt) GetPos() Pos      { return s.P }
func (s *ForStmt) stmtNode()        {}

// ReturnStmt represents a return statement: return expr, expr, ...
type ReturnStmt struct {
	P      Pos
	Values []Expr
}

func (s *ReturnStmt) nodeType() string { return "ReturnStmt" }
func (s *ReturnStmt) GetPos() Pos      { return s.P }
func (s *ReturnStmt) stmtNode()        {}

// BreakStmt represents a break statement.
type BreakStmt struct {
	P Pos
}

func (s *BreakStmt) nodeType() string { return "BreakStmt" }
func (s *BreakStmt) GetPos() Pos      { return s.P }
func (s *BreakStmt) stmtNode()        {}

// ContinueStmt represents a continue statement.
type ContinueStmt struct {
	P Pos
}

func (s *ContinueStmt) nodeType() string { return "ContinueStmt" }
func (s *ContinueStmt) GetPos() Pos      { return s.P }
func (s *ContinueStmt) stmtNode()        {}

// LabelStmt represents a Go-style statement label: name:
type LabelStmt struct {
	P    Pos
	Name string
}

func (s *LabelStmt) nodeType() string { return "LabelStmt" }
func (s *LabelStmt) GetPos() Pos      { return s.P }
func (s *LabelStmt) stmtNode()        {}

// GotoStmt represents a Go-style goto statement: goto name
type GotoStmt struct {
	P    Pos
	Name string
}

func (s *GotoStmt) nodeType() string { return "GotoStmt" }
func (s *GotoStmt) GetPos() Pos      { return s.P }
func (s *GotoStmt) stmtNode()        {}

// FuncDeclStmt represents a top-level named function declaration: func name(params) { body }
type FuncDeclStmt struct {
	P      Pos
	Name   string
	Params []FuncParam
	Body   *BlockStmt
}

func (s *FuncDeclStmt) nodeType() string { return "FuncDeclStmt" }
func (s *FuncDeclStmt) GetPos() Pos      { return s.P }
func (s *FuncDeclStmt) stmtNode()        {}

// ToolDeclStmt represents an AI tool declaration:
// tool name(params) { body }
type ToolDeclStmt struct {
	P               Pos
	Name            string
	Params          []FuncParam
	Body            *BlockStmt
	DocComment      string
	Requires        []string
	ParamDocs       map[string]string
	ParamDocEntries []ToolParamDoc
}

func (s *ToolDeclStmt) nodeType() string { return "ToolDeclStmt" }
func (s *ToolDeclStmt) GetPos() Pos      { return s.P }
func (s *ToolDeclStmt) stmtNode()        {}

type ToolParamDoc struct {
	Name string
	Doc  string
}

// AIToolRegistry indexes module-level AI tools by declaration name for
// source-level validation and future lint passes.
type AIToolRegistry map[string]AIToolRegistryEntry

// AIToolRegistryEntry captures the source metadata attached to an AI tool
// declaration.
type AIToolRegistryEntry struct {
	Name      string
	Requires  []string
	Doc       string
	Params    []FuncParam
	ParamDocs map[string]string
	Source    Pos
}

// Lookup returns a registered AI tool entry by declaration name.
func (r AIToolRegistry) Lookup(name string) (AIToolRegistryEntry, bool) {
	entry, ok := r[name]
	return entry, ok
}

// RequiredCapabilitiesForTools returns the unique non-none capabilities
// required by statically named tools, preserving first-seen order.
func (r AIToolRegistry) RequiredCapabilitiesForTools(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range names {
		entry, ok := r.Lookup(name)
		if !ok {
			continue
		}
		for _, req := range entry.Requires {
			if req == "" || req == "none" || req == "cap.none" || seen[req] {
				continue
			}
			seen[req] = true
			out = append(out, req)
		}
	}
	return out
}

// AgentDeclStmt represents a named AI agent declaration.
type AgentDeclStmt struct {
	P      Pos
	Name   string
	Params []FuncParam
	Config []ConfigField
	Flow   *BlockStmt
}

func (s *AgentDeclStmt) nodeType() string { return "AgentDeclStmt" }
func (s *AgentDeclStmt) GetPos() Pos      { return s.P }
func (s *AgentDeclStmt) stmtNode()        {}

// AgentDefaultsDeclStmt represents module-level default agent configuration:
// agent defaults { ... }
type AgentDefaultsDeclStmt struct {
	P      Pos
	Config []ConfigField
}

func (s *AgentDefaultsDeclStmt) nodeType() string { return "AgentDefaultsDeclStmt" }
func (s *AgentDefaultsDeclStmt) GetPos() Pos      { return s.P }
func (s *AgentDefaultsDeclStmt) stmtNode()        {}

// ModelsDeclStmt represents a models { ... } declaration.
type ModelsDeclStmt struct {
	P      Pos
	Config []ConfigField
}

func (s *ModelsDeclStmt) nodeType() string { return "ModelsDeclStmt" }
func (s *ModelsDeclStmt) GetPos() Pos      { return s.P }
func (s *ModelsDeclStmt) stmtNode()        {}

// BudgetStmt represents a budget { ... } { body } statement.
type BudgetStmt struct {
	P      Pos
	Config []ConfigField
	Body   *BlockStmt
}

func (s *BudgetStmt) nodeType() string { return "BudgetStmt" }
func (s *BudgetStmt) GetPos() Pos      { return s.P }
func (s *BudgetStmt) stmtNode()        {}

// ConfigField represents a field inside agent, turn, budget, models, or
// messages blocks.
type ConfigField struct {
	P     Pos
	Key   Expr
	Value Expr
}

// BlockStmt represents a block of statements enclosed in braces.
type BlockStmt struct {
	P     Pos
	Stmts []Stmt
}

func (s *BlockStmt) nodeType() string { return "BlockStmt" }
func (s *BlockStmt) GetPos() Pos      { return s.P }
func (s *BlockStmt) stmtNode()        {}

// FuncParam represents a function parameter.
type FuncParam struct {
	Name     string
	IsVarArg bool // only the last param can be vararg (...)
}

// ============================================================
// Expressions
// ============================================================

// NumberLit represents a numeric literal: 42, 3.14
type NumberLit struct {
	P     Pos
	Value string // raw text representation
}

func (e *NumberLit) nodeType() string { return "NumberLit" }
func (e *NumberLit) GetPos() Pos      { return e.P }
func (e *NumberLit) exprNode()        {}

// StringLit represents a string literal: "hello"
type StringLit struct {
	P     Pos
	Value string // the string value (with escapes resolved)
}

func (e *StringLit) nodeType() string { return "StringLit" }
func (e *StringLit) GetPos() Pos      { return e.P }
func (e *StringLit) exprNode()        {}

// BoolLit represents a boolean literal: true, false
type BoolLit struct {
	P     Pos
	Value bool
}

func (e *BoolLit) nodeType() string { return "BoolLit" }
func (e *BoolLit) GetPos() Pos      { return e.P }
func (e *BoolLit) exprNode()        {}

// NilLit represents the nil literal.
type NilLit struct {
	P Pos
}

func (e *NilLit) nodeType() string { return "NilLit" }
func (e *NilLit) GetPos() Pos      { return e.P }
func (e *NilLit) exprNode()        {}

// VarArgExpr represents the vararg expression: ...
type VarArgExpr struct {
	P Pos
}

func (e *VarArgExpr) nodeType() string { return "VarArgExpr" }
func (e *VarArgExpr) GetPos() Pos      { return e.P }
func (e *VarArgExpr) exprNode()        {}

// IdentExpr represents a variable name.
type IdentExpr struct {
	P    Pos
	Name string
}

func (e *IdentExpr) nodeType() string { return "IdentExpr" }
func (e *IdentExpr) GetPos() Pos      { return e.P }
func (e *IdentExpr) exprNode()        {}

// BinaryExpr represents a binary expression: a OP b
type BinaryExpr struct {
	P     Pos
	Left  Expr
	Op    string // "+", "-", "*", "/", "%", "**", "..", "==", "!=", "<", "<=", ">", ">=", "&&", "||"
	Right Expr
}

func (e *BinaryExpr) nodeType() string { return "BinaryExpr" }
func (e *BinaryExpr) GetPos() Pos      { return e.P }
func (e *BinaryExpr) exprNode()        {}

// UnaryExpr represents a unary expression: -a, !a, #a
type UnaryExpr struct {
	P       Pos
	Op      string // "-", "!", "#"
	Operand Expr
}

func (e *UnaryExpr) nodeType() string { return "UnaryExpr" }
func (e *UnaryExpr) GetPos() Pos      { return e.P }
func (e *UnaryExpr) exprNode()        {}

// ParenExpr preserves explicit parentheses around an expression.
// Calls and varargs inside parentheses are adjusted to a single value in list
// contexts, matching the usual expression-list boundary.
type ParenExpr struct {
	P     Pos
	Inner Expr
}

func (e *ParenExpr) nodeType() string { return "ParenExpr" }
func (e *ParenExpr) GetPos() Pos      { return e.P }
func (e *ParenExpr) exprNode()        {}

// IndexExpr represents an index access: t[k]
type IndexExpr struct {
	P     Pos
	Table Expr
	Index Expr
}

func (e *IndexExpr) nodeType() string { return "IndexExpr" }
func (e *IndexExpr) GetPos() Pos      { return e.P }
func (e *IndexExpr) exprNode()        {}

// FieldExpr represents a field access: t.k (sugar for t["k"])
type FieldExpr struct {
	P     Pos
	Table Expr
	Field string
}

func (e *FieldExpr) nodeType() string { return "FieldExpr" }
func (e *FieldExpr) GetPos() Pos      { return e.P }
func (e *FieldExpr) exprNode()        {}

// CallExpr represents a function call: f(args)
type CallExpr struct {
	P    Pos
	Func Expr   // the function expression being called
	Args []Expr // arguments
}

func (e *CallExpr) nodeType() string { return "CallExpr" }
func (e *CallExpr) GetPos() Pos      { return e.P }
func (e *CallExpr) exprNode()        {}

// MethodCallExpr represents a Lua-style method call: t:method(args)
type MethodCallExpr struct {
	P      Pos
	Object Expr
	Method string
	Args   []Expr
}

func (e *MethodCallExpr) nodeType() string { return "MethodCallExpr" }
func (e *MethodCallExpr) GetPos() Pos      { return e.P }
func (e *MethodCallExpr) exprNode()        {}

// FuncLitExpr represents a function literal: func(params) { body }
type FuncLitExpr struct {
	P      Pos
	Params []FuncParam
	Body   *BlockStmt
}

func (e *FuncLitExpr) nodeType() string { return "FuncLitExpr" }
func (e *FuncLitExpr) GetPos() Pos      { return e.P }
func (e *FuncLitExpr) exprNode()        {}

// AgentLitExpr represents an anonymous AI agent value.
type AgentLitExpr struct {
	P      Pos
	Params []FuncParam
	Config []ConfigField
	Flow   *BlockStmt
}

func (e *AgentLitExpr) nodeType() string { return "AgentLitExpr" }
func (e *AgentLitExpr) GetPos() Pos      { return e.P }
func (e *AgentLitExpr) exprNode()        {}

// TurnExpr represents a turn { ... } expression.
type TurnExpr struct {
	P      Pos
	Config []ConfigField
}

func (e *TurnExpr) nodeType() string { return "TurnExpr" }
func (e *TurnExpr) GetPos() Pos      { return e.P }
func (e *TurnExpr) exprNode()        {}

// MessagesExpr represents a messages { ... } constructor. It is kept as a
// distinct node so later lowering can preserve message role order.
type MessagesExpr struct {
	P      Pos
	Fields []ConfigField
}

func (e *MessagesExpr) nodeType() string { return "MessagesExpr" }
func (e *MessagesExpr) GetPos() Pos      { return e.P }
func (e *MessagesExpr) exprNode()        {}

// ListLitExpr represents a list literal: [a, b, c].
type ListLitExpr struct {
	P      Pos
	Values []Expr
}

func (e *ListLitExpr) nodeType() string { return "ListLitExpr" }
func (e *ListLitExpr) GetPos() Pos      { return e.P }
func (e *ListLitExpr) exprNode()        {}

// TableLitExpr represents a table literal: {k: v, k2: v2, expr, ...}
type TableLitExpr struct {
	P      Pos
	Fields []TableField
}

func (e *TableLitExpr) nodeType() string { return "TableLitExpr" }
func (e *TableLitExpr) GetPos() Pos      { return e.P }
func (e *TableLitExpr) exprNode()        {}

// TableField represents a single field in a table literal.
type TableField struct {
	Key   Expr // nil means array-style (positional value only)
	Value Expr
}

// DenseLitExpr represents an experimental typed dense array literal:
// []f64{...} or [3]f64{...}. Len == 0 means dynamic length.
type DenseLitExpr struct {
	P      Pos
	DType  string
	Len    int
	Values []Expr
}

func (e *DenseLitExpr) nodeType() string { return "DenseLitExpr" }
func (e *DenseLitExpr) GetPos() Pos      { return e.P }
func (e *DenseLitExpr) exprNode()        {}

// RecvExpr represents a channel receive expression: <-ch
type RecvExpr struct {
	P       Pos
	Channel Expr
}

func (e *RecvExpr) nodeType() string { return "RecvExpr" }
func (e *RecvExpr) GetPos() Pos      { return e.P }
func (e *RecvExpr) exprNode()        {}

// MakeChanExpr represents a channel creation: make(chan) or make(chan, size)
type MakeChanExpr struct {
	P    Pos
	Size Expr // nil for unbuffered, or an expression for buffer size
}

func (e *MakeChanExpr) nodeType() string { return "MakeChanExpr" }
func (e *MakeChanExpr) GetPos() Pos      { return e.P }
func (e *MakeChanExpr) exprNode()        {}
