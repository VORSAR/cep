package parser

// The CEP v0.1 grammar (docs/language/grammar.lark), construct for construct, as participle v2 structs (ADR-0002 as
// amended). Each struct is one production; each tag is its right-hand side in participle's notation: 'x' a literal
// token, @@ a sub-production captured into the field, @Name a token of that type, | alternation, ? * + repetition.
// Left recursion, which participle does not support, is written as "head (op tail)*" and folded left in lower.go.
// A field named Pos receives the production's first token's position; the lowering copies it into the model's Pos
// fields for diagnostics (S2.2b). The *Tok productions hold one token, so that one token's position is kept.

type Program struct {
	Pragma     *PragmaTok   `parser:"'language' @@ ';'"`
	Statements []*Statement `parser:"@@*"`
}

type PragmaTok struct {
	Pos   Position
	Value string `parser:"@Pragma"`
}

type NameTok struct {
	Pos   Position
	Value string `parser:"@(Ident|Backticked)"`
}

// StrTok is one string: quoted, or triple-quoted (raw), as grammar.lark's STRING is in every string slot.
type StrTok struct {
	Pos   Position
	Value string `parser:"@(String|RawString)"`
}

type ParamTok struct {
	Pos   Position
	Value string `parser:"@ParamRef"`
}

type NumTok struct {
	Pos   Position
	Value string `parser:"@Number"`
}

type Statement struct {
	Param  *ParamDecl  `parser:"  @@"`
	Stream *StreamDecl `parser:"| @@"`
	Table  *TableDecl  `parser:"| @@"`
	Rule   *RuleDecl   `parser:"| @@"`
}

type ParamDecl struct {
	Pos     Position
	Name    string         `parser:"'param' @(Ident|Backticked)"`
	Type    string         `parser:"@('string'|'int'|'float'|'bool'|'timestamp'|'duration'|'object')"`
	Default *SignedLiteral `parser:"'=' @@ ';'"`
}

type StreamDecl struct {
	Pos         Position
	Annotations []*Annotation `parser:"@@*"`
	Name        string        `parser:"'stream' @(Ident|Backticked)"`
	Attrs       []*Attr       `parser:"'(' @@ (',' @@)* ')'"`
	Time        *TimeClause   `parser:"@@? ';'"`
}

type Attr struct {
	Pos      Position
	Name     string `parser:"@(Ident|Backticked)"`
	Type     string `parser:"@('string'|'int'|'float'|'bool'|'timestamp'|'duration'|'object')"`
	Optional bool   `parser:"@'optional'?"`
}

type TimeClause struct {
	Pos       Position
	Arrival   bool      `parser:"'time' ( @'arrival'"`
	Attr      *NameTok  `parser:"| @@ 'watermark'"`
	Bounded   *Duration `parser:"  ( 'bounded' @@"`
	Ascending bool      `parser:"  | @'ascending' ) )"`
	Idle      *Duration `parser:"('idle' 'after' @@)?"`
}

type Annotation struct {
	Pos   Position
	Name  string     `parser:"'@' @(Ident|Backticked)"`
	Items []*AnnItem `parser:"'(' @@ (',' @@)* ')'"`
}

type AnnItem struct {
	Pos    Position
	Nested *Annotation `parser:"  @@"`
	Key    string      `parser:"| @(Ident|Backticked) '='"`
	Value  *Literal    `parser:"  ( @@"`
	Param  *ParamTok   `parser:"  | @@ )"`
}

type TableDecl struct {
	Pos        Position
	Name       string   `parser:"'table' @(Ident|Backticked)"`
	Attrs      []*Attr  `parser:"'(' @@ (',' @@)* ')'"`
	PrimaryKey []string `parser:"'primary' 'key' '(' @(Ident|Backticked) (',' @(Ident|Backticked))* ')' ';'"`
}

type RuleDecl struct {
	Pos        Position
	Name       string           `parser:"'rule' @(Ident|Backticked)"`
	Intent     *StrTok          `parser:"'intent' @@"`
	Partition  *Partition       `parser:"@@?"`
	Body       *Body            `parser:"@@"`
	Extras     []*Extra         `parser:"@@*"`
	Select     *SelectClause    `parser:"@@"`
	Rate       *DurationOrParam `parser:"('output' 'first' 'every' @@)?"`
	InsertInto *NameTok         `parser:"'insert' 'into' @@ ';'"`
}

type SelectClause struct {
	Pos   Position
	Items []*SelItem `parser:"'select' @@ (',' @@)*"`
}

type Partition struct {
	Pos Position
	PartitionKeys
	TTL *DurationOrParam `parser:"'ttl' @@"`
}

// PartitionKeys is the partition clause's head: `partition by` and its keys. Its leading literals capture nothing,
// so they sit in a field of their own that holds no value (participle matches a tag with no capture and assigns
// nothing); embedded, its fields are Partition's and participle names no production for it.
type PartitionKeys struct {
	By   struct{}   `parser:"'partition' 'by'"`
	Keys []*PartKey `parser:"@@ (',' @@)*"`
}

// PartKey is `k of S` or `(k, j, …) of S` (SPEC §4.4, rev 5). The grammar accepts a one-attribute group, which the
// validator answers with E-PART-005 and its fix (design s3-2b-2 §1.2).
type PartKey struct {
	Pos    Position
	Group  *KeyGroup `parser:"( @@"`
	Attr   *NameTok  `parser:"| @@ )"`
	Stream *NameTok  `parser:"'of' @@"`
}

// KeyGroup is the bracketed attribute list of a grouped key.
type KeyGroup struct {
	Pos   Position
	Attrs []*NameTok `parser:"'(' @@ (',' @@)* ')'"`
}

type Body struct {
	Pos     Position
	Pattern *PatternBody `parser:"  'from' @@"`
	Source  *Source      `parser:"| 'from' @@"`
	Window  *WindowKind  `parser:"  ( 'window' @@"`
	Emit    *EmitTok     `parser:"    'emit' 'on' @@"`
	Late    *LateClause  `parser:"    'late' 'events' @@ )?"`
}

type EmitTok struct {
	Pos   Position
	Value string `parser:"@('close'|'change'|'due'|'batch' 'end')"`
}

type PatternBody struct {
	PatternChain
	WithinClause
	AfterMatch bool        `parser:"@('after' 'match' 'skip' 'past' 'last')"`
	Late       *LateClause `parser:"'late' 'events' @@"`
}

// PatternChain is a pattern's restart marker and its steps: the first positive step, then each `-> step` or
// `and not S[…] for D`. Embedded, its fields are the pattern body's.
type PatternChain struct {
	Restart string       `parser:"@('every'|'once')"`
	First   *PosElem     `parser:"@@"`
	Chain   []*ChainElem `parser:"@@*"`
}

// WithinClause is a pattern's bound: `within D`.
type WithinClause struct {
	Within *DurationOrParam `parser:"'within' @@"`
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
	Pos    Position
	Name   string  `parser:"@(Ident|Backticked) '='"`
	Source *Source `parser:"@@"`
}

type Source struct {
	Pos    Position
	Stream string `parser:"@(Ident|Backticked)"`
	Filter *Expr  `parser:"('[' @@ ']')?"`
}

type WindowKind struct {
	Pos      Position
	SortN    *NumTok          `parser:"  'sort' '(' @@"`
	SortAttr *NameTok         `parser:"    ',' @@"`
	SortDir  string           `parser:"    ',' @('asc'|'desc') ')'"`
	CronExpr *StrTok          `parser:"| 'cron' '(' @@"`
	CronTZ   *StrTok          `parser:"    ',' @@ ')'"`
	Delay    *DurationOrParam `parser:"| 'delay' '(' @@ ')'"`
	Batch    bool             `parser:"| @('batch' '(' ')')"`
}

type LateClause struct {
	Pos   Position
	Drop  bool             `parser:"  @'drop'"`
	Allow *DurationOrParam `parser:"| 'allow' @@"`
}

type Extra struct {
	Pos    Position
	Expect *ExpectBlock `parser:"  'expect' @@"`
	Todo   *StrTok      `parser:"| 'todo' @@"`
}

type ExpectBlock struct {
	Givens []*GivenSec `parser:"@@+"`
	Then   *ThenSec    `parser:"@@"`
}

type GivenSec struct {
	Pos     Position
	Table   bool      `parser:"'given' @'table'?"`
	Name    *NameTok  `parser:"@@"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type ThenSec struct {
	Pos     Position
	Name    *NameTok  `parser:"'then' @@"`
	Records []*Record `parser:"'[' (@@ (',' @@)*)? ']'"`
}

type Record struct {
	Pos    Position
	Fields []*Field `parser:"'{' @@ (',' @@)* '}'"`
}

type Field struct {
	Pos   Position
	Name  string      `parser:"@(Ident|Backticked) ':'"`
	Value *FieldValue `parser:"@@"`
}

// FieldValue is a record field's value: `null` or a signed literal, with the position of its first token (design
// s3-3-2 §2).
type FieldValue struct {
	Pos  Position
	Null bool           `parser:"( @'null'"`
	Lit  *SignedLiteral `parser:"| @@ )"`
}

type SelItem struct {
	Pos  Position
	Expr *Expr  `parser:"@@"`
	As   string `parser:"('as' @(Ident|Backticked))?"`
}

// ---- expressions: precedence low to high, left-recursion as repetition ----

type Expr struct {
	Pos  Position
	Left *AndExpr   `parser:"@@"`
	Rest []*AndExpr `parser:"('or' @@)*"`
}

type AndExpr struct {
	Pos  Position
	Left *NotExpr   `parser:"@@"`
	Rest []*NotExpr `parser:"('and' @@)*"`
}

type NotExpr struct {
	Pos    Position
	Not    *NotExpr    `parser:"  'not' @@"`
	Member *Membership `parser:"| @@"`
}

type Membership struct {
	Pos   Position
	Cmp   *Comparison `parser:"@@"`
	NotIn bool        `parser:"( @'not'? 'in'"`
	Table *NameTok    `parser:"  @@ )?"`
}

type Comparison struct {
	Pos  Position
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
	Pos    Position
	String string `parser:"  @(String|RawString)"`
	Param  string `parser:"| @ParamRef"`
}

type SetOperand struct {
	Pos     Position
	Strings []*StrTok `parser:"  @@ (',' @@)*"`
	Param   string    `parser:"| @ParamRef"`
	Table   string    `parser:"| @(Ident|Backticked)"`
}

type Sum struct {
	Pos  Position
	Left *Term      `parser:"@@"`
	Rest []*SumRest `parser:"@@*"`
}

type SumRest struct {
	Op   string `parser:"@('+'|'-')"`
	Term *Term  `parser:"@@"`
}

type Term struct {
	Pos  Position
	Left *Atom       `parser:"@@"`
	Rest []*TermRest `parser:"@@*"`
}

type TermRest struct {
	Op   string `parser:"@('*'|'/'|'%')"`
	Atom *Atom  `parser:"@@"`
}

type Atom struct {
	Pos     Position
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
	Timestamp string    `parser:"| 'timestamp' @(String|RawString)"`
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
	Pos      Position
	Duration *Duration `parser:"  @@"`
	Param    string    `parser:"| @ParamRef"`
}
