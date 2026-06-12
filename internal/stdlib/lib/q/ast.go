package q

type QueryKind string

const (
	SelectQuery QueryKind = "select"
	ExecQuery   QueryKind = "exec"
	UpdateQuery QueryKind = "update"
	DeleteQuery QueryKind = "delete"
	InsertQuery QueryKind = "insert"
	UpsertQuery QueryKind = "upsert"
)

type Expr interface {
	exprNode()
}

type Query struct {
	Kind     QueryKind
	Distinct bool
	Columns  []Column
	Values   []Expr
	By       []Column
	From     string
	FromExpr Expr
	// FromQuery is a parenthesized sub-select in from position; the outer
	// query executes against the subquery's result.
	FromQuery *Query
	Join      *Join
	Joins     []Join
	Where     Expr
	OrderBy   []OrderTerm
	Limit     *int
	Take      *int
}

type Join struct {
	Kind   string
	Right  string
	Keys   []JoinKey
	Window *WindowBounds
}

type JoinKey struct {
	Left  string
	Right string
}

type WindowBounds struct {
	Low  Expr
	High Expr
}

type Column struct {
	Name string
	Expr Expr
}

type OrderTerm struct {
	Column string
	Expr   Expr
	Desc   bool
}

type Ident struct {
	Name string
}

type Number struct {
	Text string
}

type String struct {
	Value string
}

type Symbol struct {
	Name string
}

type Bool struct {
	Value bool
}

type Null struct{}

type AllColumns struct{}

type Temporal struct {
	Kind string
	Text string
}

type TypedNull struct {
	Kind string
}

type Binary struct {
	Op    string
	Left  Expr
	Right Expr
}

type Call struct {
	Func string
	Arg  Expr
}

type Vector struct {
	Items []Expr
}

type DictExpr struct {
	Keys   Expr
	Values Expr
}

type IndexExpr struct {
	Expr  Expr
	Index Expr
}

type Flip struct {
	Keys    []Column
	Columns []Column
}

func (Ident) exprNode()      {}
func (Number) exprNode()     {}
func (String) exprNode()     {}
func (Symbol) exprNode()     {}
func (Bool) exprNode()       {}
func (Null) exprNode()       {}
func (AllColumns) exprNode() {}
func (Temporal) exprNode()   {}
func (TypedNull) exprNode()  {}
func (Binary) exprNode()     {}
func (Call) exprNode()       {}
func (Vector) exprNode()     {}
func (DictExpr) exprNode()   {}
func (IndexExpr) exprNode()  {}
func (Flip) exprNode()       {}
