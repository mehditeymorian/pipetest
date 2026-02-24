package ast

// Position represents a specific point in a source file.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span represents a half-open source range.
type Span struct {
	Start Position
	End   Position
}

// Program is the root AST node.
type Program struct {
	Stmts []Stmt
	Span  Span
}

// Stmt marks top-level statements.
type Stmt interface {
	stmtNode()
}

// ReqLine marks statements inside a request block.
type ReqLine interface {
	reqLineNode()
}

// HookStmt marks statements inside a hook block.
type HookStmt interface {
	hookStmtNode()
}

// Expr marks expression nodes.
type Expr interface {
	exprNode()
}

// SettingKind identifies a base or timeout setting.
type SettingKind int

const (
	SettingBase SettingKind = iota
	SettingTimeout
)

// SettingStmt represents a base/timeout setting.
type SettingStmt struct {
	Kind  SettingKind
	Value Literal
	Span  Span
}

func (*SettingStmt) stmtNode() {}

// ImportStmt represents an import statement.
type ImportStmt struct {
	Path *StringLit
	Span Span
}

func (*ImportStmt) stmtNode() {}

// LetStmt binds a name to an expression.
type LetStmt struct {
	Name  string
	Value Expr
	Span  Span
}

func (*LetStmt) stmtNode()     {}
func (*LetStmt) reqLineNode()  {}
func (*LetStmt) hookStmtNode() {}

// ReqDecl declares a request block.
type ReqDecl struct {
	Name   string
	Parent *string
	Lines  []ReqLine
	Span   Span
}

func (*ReqDecl) stmtNode() {}

// FlowDecl declares a flow block.
type FlowDecl struct {
	Name    *StringLit
	Prelude []*LetStmt
	Chain   []FlowStep
	Asserts []*AssertStmt
	Span    Span
}

func (*FlowDecl) stmtNode() {}

// FlowStep references a request with an optional alias.
type FlowStep struct {
	ReqName string
	Alias   *string
	Span    Span
}

// HttpMethod identifies an HTTP method.
type HttpMethod int

const (
	MethodGet HttpMethod = iota
	MethodPost
	MethodPut
	MethodPatch
	MethodDelete
	MethodHead
	MethodOptions
)

// HttpLine is a request HTTP line.
type HttpLine struct {
	Method HttpMethod
	Path   string
	Span   Span
}

func (*HttpLine) reqLineNode() {}

// Directive marks request directives.
type Directive interface {
	ReqLine
	directiveNode()
}

// JsonDirective sets a JSON body.
type JsonDirective struct {
	Value *ObjectLit
	Span  Span
}

func (*JsonDirective) reqLineNode()   {}
func (*JsonDirective) directiveNode() {}

// HeaderDirective sets a header.
type HeaderDirective struct {
	Key   Key
	Value Expr
	Span  Span
}

func (*HeaderDirective) reqLineNode()   {}
func (*HeaderDirective) directiveNode() {}

// QueryDirective sets a query parameter.
type QueryDirective struct {
	Key   Key
	Value Expr
	Span  Span
}

func (*QueryDirective) reqLineNode()   {}
func (*QueryDirective) directiveNode() {}

// AuthScheme identifies supported auth schemes.
type AuthScheme int

const (
	AuthBearer AuthScheme = iota
)

// AuthDirective sets authorization configuration.
type AuthDirective struct {
	Scheme AuthScheme
	Value  Expr
	Span   Span
}

func (*AuthDirective) reqLineNode()   {}
func (*AuthDirective) directiveNode() {}

// HookKind identifies hook type.
type HookKind int

const (
	HookPre HookKind = iota
	HookPost
)

// HookBlock represents a pre/post hook block.
type HookBlock struct {
	Kind  HookKind
	Stmts []HookStmt
	Span  Span
}

func (*HookBlock) reqLineNode() {}

// AssertStmt represents a ? assertion line.
type AssertStmt struct {
	Expr Expr
	Span Span
}

func (*AssertStmt) reqLineNode() {}

// AssignStmt represents a hook assignment.
type AssignStmt struct {
	Target *LValue
	Value  Expr
	Span   Span
}

func (*AssignStmt) hookStmtNode() {}

// ExprStmt represents a bare expression statement in hooks.
type ExprStmt struct {
	Expr Expr
	Span Span
}

func (*ExprStmt) hookStmtNode() {}

// KeyKind distinguishes key token forms.
type KeyKind int

const (
	KeyIdent KeyKind = iota
	KeyBare
	KeyString
)

// Key represents a header/query key.
type Key struct {
	Kind KeyKind
	Name string
	Raw  string
	Span Span
}

// ObjectKeyKind distinguishes object literal key forms.
type ObjectKeyKind int

const (
	ObjectKeyIdent ObjectKeyKind = iota
	ObjectKeyString
)

// ObjectKey represents an object literal key.
type ObjectKey struct {
	Kind ObjectKeyKind
	Name string
	Raw  string
	Span Span
}

// ObjectPair is a key/value pair in an object literal.
type ObjectPair struct {
	Key   ObjectKey
	Value Expr
	Span  Span
}

// LValue represents an assignment target in hooks.
type LValue struct {
	Root    LValueRoot
	Postfix []LValuePostfix
	Span    Span
}

// LValueRootKind identifies the root of an assignment target.
type LValueRootKind int

const (
	LValueIdent LValueRootKind = iota
	LValueReq
	LValueRes
	LValueDollar
)

// LValueRoot is the first element of an assignment target.
type LValueRoot struct {
	Kind LValueRootKind
	Name string
	Span Span
}

// LValuePostfixKind identifies postfix operations on LValues.
type LValuePostfixKind int

const (
	LValueField LValuePostfixKind = iota
	LValueIndex
)

// LValuePostfix represents a field/index access in an assignment target.
type LValuePostfix struct {
	Kind  LValuePostfixKind
	Name  string
	Index Expr
	Span  Span
}

// Literal marks literal expressions.
type Literal interface {
	Expr
	literalNode()
}

// IdentExpr is an identifier reference.
type IdentExpr struct {
	Name string
	Span Span
}

func (*IdentExpr) exprNode() {}

// StringLit is a string literal.
type StringLit struct {
	Raw   string
	Value string
	Span  Span
}

func (*StringLit) exprNode()    {}
func (*StringLit) literalNode() {}

// NumberLit is a numeric literal.
type NumberLit struct {
	Raw  string
	Span Span
}

func (*NumberLit) exprNode()    {}
func (*NumberLit) literalNode() {}

// DurationLit is a duration literal.
type DurationLit struct {
	Raw  string
	Span Span
}

func (*DurationLit) exprNode()    {}
func (*DurationLit) literalNode() {}

// BoolLit is a boolean literal.
type BoolLit struct {
	Value bool
	Span  Span
}

func (*BoolLit) exprNode()    {}
func (*BoolLit) literalNode() {}

// NullLit is a null literal.
type NullLit struct {
	Span Span
}

func (*NullLit) exprNode()    {}
func (*NullLit) literalNode() {}

// DollarExpr references the current JSON root.
type DollarExpr struct {
	Span Span
}

func (*DollarExpr) exprNode() {}

// ArrayLit is an array literal.
type ArrayLit struct {
	Elements []Expr
	Span     Span
}

func (*ArrayLit) exprNode()    {}
func (*ArrayLit) literalNode() {}

// ObjectLit is an object literal.
type ObjectLit struct {
	Pairs []ObjectPair
	Span  Span
}

func (*ObjectLit) exprNode()    {}
func (*ObjectLit) literalNode() {}

// UnaryOp identifies a unary operator.
type UnaryOp int

const (
	UnaryNot UnaryOp = iota
	UnaryPlus
	UnaryMinus
)

// UnaryExpr applies a unary operator.
type UnaryExpr struct {
	Op   UnaryOp
	X    Expr
	Span Span
}

func (*UnaryExpr) exprNode() {}

// BinaryOp identifies a binary operator.
type BinaryOp int

const (
	BinaryOr BinaryOp = iota
	BinaryAnd
	BinaryEq
	BinaryNe
	BinaryLt
	BinaryLte
	BinaryGt
	BinaryGte
	BinaryIn
	BinaryContains
	BinaryMatch
	BinaryAdd
	BinarySub
	BinaryMul
	BinaryDiv
	BinaryMod
)

// BinaryExpr applies a binary operator.
type BinaryExpr struct {
	Op    BinaryOp
	Left  Expr
	Right Expr
	Span  Span
}

func (*BinaryExpr) exprNode() {}

// CallExpr represents a function call.
type CallExpr struct {
	Callee Expr
	Args   []Expr
	Span   Span
}

func (*CallExpr) exprNode() {}

// FieldExpr represents a field access.
type FieldExpr struct {
	X    Expr
	Name string
	Span Span
}

func (*FieldExpr) exprNode() {}

// IndexExpr represents an index access.
type IndexExpr struct {
	X     Expr
	Index Expr
	Span  Span
}

func (*IndexExpr) exprNode() {}

// ParenExpr wraps a parenthesized expression.
type ParenExpr struct {
	X    Expr
	Span Span
}

func (*ParenExpr) exprNode() {}

// BadExpr is a placeholder for parse errors.
type BadExpr struct {
	Span Span
}

func (*BadExpr) exprNode() {}
