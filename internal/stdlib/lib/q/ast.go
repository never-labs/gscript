package q

type QueryKind string

const (
	SelectQuery QueryKind = "select"
	ExecQuery   QueryKind = "exec"
)

type Expr interface {
	exprNode()
}

type Query struct {
	Kind    QueryKind
	Columns []Column
	By      []Expr
	From    string
	Where   Expr
	OrderBy []OrderTerm
	Limit   *int
}

type Column struct {
	Name string
	Expr Expr
}

type OrderTerm struct {
	Column string
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

type Flip struct {
	Columns []Column
}

func (Ident) exprNode()  {}
func (Number) exprNode() {}
func (String) exprNode() {}
func (Symbol) exprNode() {}
func (Bool) exprNode()   {}
func (Null) exprNode()   {}
func (Binary) exprNode() {}
func (Call) exprNode()   {}
func (Vector) exprNode() {}
func (Flip) exprNode()   {}
