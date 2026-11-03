package parser

// The CEP v0.1 grammar (docs/language/grammar.lark), construct for construct, as participle v2 structs (ADR-0002 as
// amended). Each struct is one production; each tag is its right-hand side in participle's notation: 'x' a literal
// token, @@ a sub-production captured into the field, @Name a token of that type, | alternation, ? * + repetition.
// Left recursion, which participle does not support, is written as "head (op tail)*" and folded left in lower.go.
// A field named Pos receives the production's first token's position; the model keeps none of them.

type Program struct {
	Pragma     string       `parser:"'language' @Pragma ';'"`
	Statements []*Statement `parser:"@@*"`
}

type Statement struct {
	Param  *ParamDecl  `parser:"  @@"`
	Stream *StreamDecl `parser:"| @@"`
	Table  *TableDecl  `parser:"| @@"`
	Rule   *RuleDecl   `parser:"| @@"`
}

type ParamDecl struct {
	Name    string         `parser:"'param' @(Ident|Backticked)"`
	Type    string         `parser:"@('string'|'int'|'float'|'bool'|'timestamp'|'duration'|'object')"`
	Default *SignedLiteral `parser:"'=' @@ ';'"`
}

type StreamDecl struct {
	Annotations []*Annotation `parser:"@@*"`
	Name        string        `parser:"'stream' @(Ident|Backticked)"`
	Attrs       []*Attr       `parser:"'(' @@ (',' @@)* ')'"`
	Time        *TimeClause   `parser:"@@? ';'"`
}

type Attr struct {
	Name     string `parser:"@(Ident|Backticked)"`
	Type     string `parser:"@('string'|'int'|'float'|'bool'|'timestamp'|'duration'|'object')"`
	Optional bool   `parser:"@'optional'?"`
}

type TimeClause struct {
	Arrival   bool      `parser:"'time' ( @'arrival'"`
	Attr      string    `parser:"| @(Ident|Backticked) 'watermark'"`
	Bounded   *Duration `parser:"  ( 'bounded' @@"`
	Ascending bool      `parser:"  | @'ascending' ) )"`
	Idle      *Duration `parser:"('idle' 'after' @@)?"`
}

type Annotation struct {
	Name  string     `parser:"'@' @(Ident|Backticked)"`
	Items []*AnnItem `parser:"'(' @@ (',' @@)* ')'"`
}

type AnnItem struct {
	Nested *Annotation `parser:"  @@"`
	Key    string      `parser:"| @(Ident|Backticked) '='"`
	Value  *Literal    `parser:"  ( @@"`
	Param  string      `parser:"  | @ParamRef )"`
}

type TableDecl struct {
	Name       string   `parser:"'table' @(Ident|Backticked)"`
	Attrs      []*Attr  `parser:"'(' @@ (',' @@)* ')'"`
	PrimaryKey []string `parser:"'primary' 'key' '(' @(Ident|Backticked) (',' @(Ident|Backticked))* ')' ';'"`
}

type RuleDecl struct {
	Pos        Position
	Name       string           `parser:"'rule' @(Ident|Backticked)"`
	Intent     string           `parser:"'intent' @(String|RawString)"`
	Partition  *Partition       `parser:"@@?"`
	Body       *Body            `parser:"@@"`
	Extras     []*Extra         `parser:"@@*"`
	Select     []*SelItem       `parser:"'select' @@ (',' @@)*"`
	Rate       *DurationOrParam `parser:"('output' 'first' 'every' @@)?"`
	InsertInto string           `parser:"'insert' 'into' @(Ident|Backticked) ';'"`
}

type Partition struct {
	Keys []*PartKey       `parser:"'partition' 'by' @@ (',' @@)*"`
	TTL  *DurationOrParam `parser:"'ttl' @@"`
}

type PartKey struct {
	Attr   string `parser:"@(Ident|Backticked)"`
	Stream string `parser:"'of' @(Ident|Backticked)"`
}

type Body struct {
	Pattern *PatternBody `parser:"  'from' @@"`
	Source  *Source      `parser:"| 'from' @@"`
	Window  *WindowKind  `parser:"  ( 'window' @@"`
	Emit    string       `parser:"    'emit' 'on' @('close'|'change'|'due'|'batch' 'end')"`
	Late    *LateClause  `parser:"    'late' 'events' @@ )?"`
}

type PatternBody struct {
	Restart    string           `parser:"@('every'|'once')"`
	First      *PosElem         `parser:"@@"`
	Chain      []*ChainElem     `parser:"@@*"`
	Within     *DurationOrParam `parser:"'within' @@"`
	AfterMatch bool             `parser:"@('after' 'match' 'skip' 'past' 'last')"`
	Late       *LateClause      `parser:"'late' 'events' @@"`
}

type ChainElem struct {
	Followed *PosElem         `parser:"  '->' @@"`
	Absence  *Source          `parser:"| 'and' 'not' @@"`
	For      *DurationOrParam `parser:"  'for' @@"`
}

type PosElem struct {
	Step  *Step `parser:"  @@"`
	Left  *Step `parser:"| '(' @@"`
	Right *Step `parser:"  'and' @@ ')'"`
}

type Step struct {
	Name   string  `parser:"@(Ident|Backticked) '='"`
	Source *Source `parser:"@@"`
}

type Source struct {
	Stream string `parser:"@(Ident|Backticked)"`
	Filter *Expr  `parser:"('[' @@ ']')?"`
}

type WindowKind struct {
	Pos      Position
	SortN    string           `parser:"  'sort' '(' @Number"`
	SortAttr string           `parser:"    ',' @(Ident|Backticked)"`
	SortDir  string           `parser:"    ',' @('asc'|'desc') ')'"`
	CronExpr string           `parser:"| 'cron' '(' @String"`
	CronTZ   string           `parser:"    ',' @String ')'"`
	Delay    *DurationOrParam `parser:"| 'delay' '(' @@ ')'"`
	Batch    bool             `parser:"| @('batch' '(' ')')"`
}

type LateClause struct {
	Drop  bool             `parser:"  @'drop'"`
	Allow *DurationOrParam `parser:"| 'allow' @@"`
}

type Extra struct {
	Expect *ExpectBlock `parser:"  'expect' @@"`
	Todo   string       `parser:"| 'todo' @String"`
}

type ExpectBlock struct {
	Givens []*GivenSec `parser:"@@+"`
	Then   *ThenSec    `parser:"@@"`
}

type GivenSec struct {
	Table   bool      `parser:"'given' @'table'?"`
	Name    string    `parser:"@(Ident|Backticked)"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type ThenSec struct {
	Name    string    `parser:"'then' @(Ident|Backticked)"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type Record struct {
	Fields []*Field `parser:"'{' @@ (',' @@)* '}'"`
}

type Field struct {
	Name  string         `parser:"@(Ident|Backticked) ':'"`
	Null  bool           `parser:"( @'null'"`
	Value *SignedLiteral `parser:"| @@ )"`
}

type SelItem struct {
	Expr *Expr  `parser:"@@"`
	As   string `parser:"('as' @(Ident|Backticked))?"`
}

// ---- expressions: precedence low to high, left-recursion as repetition ----

type Expr struct {
	Left *AndExpr   `parser:"@@"`
	Rest []*AndExpr `parser:"('or' @@)*"`
}

type AndExpr struct {
	Left *NotExpr   `parser:"@@"`
	Rest []*NotExpr `parser:"('and' @@)*"`
}

type NotExpr struct {
	Not    *NotExpr    `parser:"  'not' @@"`
	Member *Membership `parser:"| @@"`
}

type Membership struct {
	Cmp   *Comparison `parser:"@@"`
	NotIn bool        `parser:"( @'not'? 'in'"`
	Table string      `parser:"  @(Ident|Backticked) )?"`
}

type Comparison struct {
	Left *Sum     `parser:"@@"`
	Tail *CmpTail `parser:"@@?"`
}

type CmpTail struct {
	Pos     Position
	Op      string       `parser:"  @Cmp"`
	Right   *Sum         `parser:"  @@"`
	IsNull  bool         `parser:"| 'is' ( @'null'"`
	NotNull bool         `parser:"       | @('not' 'null') )"`
	TextOp  string       `parser:"| @('has_ci'|'has_cs'|'contains_ci'|'contains_cs'|'starts_with_ci'|'starts_with_cs'|'ends_with_ci'|'ends_with_cs'|'matches_ci'|'matches_cs'|'contains_match_ci'|'contains_match_cs')"`
	Operand *TextOperand `parser:"  @@"`
	SetOp   string       `parser:"| @('has_any_ci'|'has_any_cs'|'has_all_ci'|'has_all_cs'|'contains_any_ci'|'contains_any_cs'|'contains_all_ci'|'contains_all_cs')"`
	SetArg  *SetOperand  `parser:"  '(' @@ ')'"`
}

type TextOperand struct {
	String string `parser:"  @(String|RawString)"`
	Param  string `parser:"| @ParamRef"`
}

type SetOperand struct {
	Strings []string `parser:"  @(String|RawString) (',' @(String|RawString))*"`
	Param   string   `parser:"| @ParamRef"`
	Table   string   `parser:"| @(Ident|Backticked)"`
}

type Sum struct {
	Left *Term      `parser:"@@"`
	Rest []*SumRest `parser:"@@*"`
}

type SumRest struct {
	Op   string `parser:"@('+'|'-')"`
	Term *Term  `parser:"@@"`
}

type Term struct {
	Left *Atom       `parser:"@@"`
	Rest []*TermRest `parser:"@@*"`
}

type TermRest struct {
	Op   string `parser:"@('*'|'/'|'%')"`
	Atom *Atom  `parser:"@@"`
}

type Atom struct {
	Literal *Literal  `parser:"  @@"`
	Param   string    `parser:"| @ParamRef"`
	Raw     *AttrRef  `parser:"| 'raw' '(' @@ ')'"`
	Call    *FuncCall `parser:"| @@"`
	Attr    *AttrRef  `parser:"| @@"`
	Neg     *Atom     `parser:"| '-' @@"`
	Paren   *Expr     `parser:"| '(' @@ ')'"`
}

type FuncCall struct {
	Namespace string  `parser:"(@(Ident|Backticked) ':')?"`
	Name      string  `parser:"@(Ident|Backticked)"`
	Args      []*Expr `parser:"'(' (@@ (',' @@)*)? ')'"`
}

type AttrRef struct {
	Qualifier string `parser:"(@(Ident|Backticked) '.')?"`
	Name      string `parser:"@(Ident|Backticked)"`
}

type Literal struct {
	Pos       Position
	String    string    `parser:"  @String"`
	Raw       string    `parser:"| @RawString"`
	Duration  *Duration `parser:"| @@"`
	Number    string    `parser:"| @Number"`
	True      bool      `parser:"| @'true'"`
	False     bool      `parser:"| @'false'"`
	Timestamp string    `parser:"| 'timestamp' @String"`
}

type SignedLiteral struct {
	Pos     Position
	Neg     string   `parser:"  '-' @Number"`
	Literal *Literal `parser:"| @@"`
}

type Duration struct {
	Parts []*DurPart `parser:"@@+"`
}

type DurPart struct {
	Pos   Position
	Count string `parser:"@Number"`
	Unit  string `parser:"@('ms'|'sec'|'min'|'hour'|'day'|'week')"`
}

type DurationOrParam struct {
	Duration *Duration `parser:"  @@"`
	Param    string    `parser:"| @ParamRef"`
}
