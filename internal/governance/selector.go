package governance

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/DocHoax/watchdog/pkg/model"
)

const (
	// MaxSelectorLength is the maximum allowed length of a selector query string.
	MaxSelectorLength = 1024
	// MaxSelectorDepth is the maximum allowed AST nesting depth to guard against recursion limits.
	MaxSelectorDepth = 10
	// MaxSelectorTokens is the maximum number of tokens allowed in a single query.
	MaxSelectorTokens = 128
)

// TokenType represents lexical token categories.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenString
	TokenNumber
	TokenBool
	TokenEq        // == or =
	TokenNeq       // !=
	TokenGt        // >
	TokenGte       // >=
	TokenLt        // <
	TokenLte       // <=
	TokenIn        // in
	TokenNotIn     // notin or "not in"
	TokenExists    // exists
	TokenNotExists // not_exists
	TokenAnd       // AND or &&
	TokenOr        // OR or ||
	TokenNot       // NOT or !
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenComma     // ,
)

// Token captures a single lexical element.
type Token struct {
	Type     TokenType
	Literal  string
	Position int
}

// Lexer converts a selector string into a slice of tokens.
type Lexer struct {
	input []rune
	pos   int
}

// NewLexer initializes a lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: []rune(input), pos: 0}
}

func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token

	for l.pos < len(l.input) {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}

		if len(tokens) >= MaxSelectorTokens {
			return nil, fmt.Errorf("%w: selector exceeds maximum token count of %d", ErrInvalidSelector, MaxSelectorTokens)
		}

		ch := l.input[l.pos]
		startPos := l.pos

		switch {
		case ch == '(':
			tokens = append(tokens, Token{Type: TokenLParen, Literal: "(", Position: startPos})
			l.pos++
		case ch == ')':
			tokens = append(tokens, Token{Type: TokenRParen, Literal: ")", Position: startPos})
			l.pos++
		case ch == '[':
			tokens = append(tokens, Token{Type: TokenLBracket, Literal: "[", Position: startPos})
			l.pos++
		case ch == ']':
			tokens = append(tokens, Token{Type: TokenRBracket, Literal: "]", Position: startPos})
			l.pos++
		case ch == ',':
			tokens = append(tokens, Token{Type: TokenComma, Literal: ",", Position: startPos})
			l.pos++
		case ch == '=':
			if l.peek() == '=' {
				tokens = append(tokens, Token{Type: TokenEq, Literal: "==", Position: startPos})
				l.pos += 2
			} else {
				tokens = append(tokens, Token{Type: TokenEq, Literal: "==", Position: startPos})
				l.pos++
			}
		case ch == '!':
			if l.peek() == '=' {
				tokens = append(tokens, Token{Type: TokenNeq, Literal: "!=", Position: startPos})
				l.pos += 2
			} else {
				tokens = append(tokens, Token{Type: TokenNot, Literal: "!", Position: startPos})
				l.pos++
			}
		case ch == '>':
			if l.peek() == '=' {
				tokens = append(tokens, Token{Type: TokenGte, Literal: ">=", Position: startPos})
				l.pos += 2
			} else {
				tokens = append(tokens, Token{Type: TokenGt, Literal: ">", Position: startPos})
				l.pos++
			}
		case ch == '<':
			if l.peek() == '=' {
				tokens = append(tokens, Token{Type: TokenLte, Literal: "<=", Position: startPos})
				l.pos += 2
			} else {
				tokens = append(tokens, Token{Type: TokenLt, Literal: "<", Position: startPos})
				l.pos++
			}
		case ch == '&' && l.peek() == '&':
			tokens = append(tokens, Token{Type: TokenAnd, Literal: "&&", Position: startPos})
			l.pos += 2
		case ch == '|' && l.peek() == '|':
			tokens = append(tokens, Token{Type: TokenOr, Literal: "||", Position: startPos})
			l.pos += 2
		case ch == '"' || ch == '\'':
			str, err := l.readString(ch)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Type: TokenString, Literal: str, Position: startPos})
		case unicode.IsDigit(ch) || (ch == '-' && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1])):
			num := l.readNumber()
			tokens = append(tokens, Token{Type: TokenNumber, Literal: num, Position: startPos})
		case isIdentStart(ch):
			ident := l.readIdent()
			lower := strings.ToLower(ident)
			switch lower {
			case "and":
				tokens = append(tokens, Token{Type: TokenAnd, Literal: ident, Position: startPos})
			case "or":
				tokens = append(tokens, Token{Type: TokenOr, Literal: ident, Position: startPos})
			case "not":
				// Look ahead for "not in" or "not exists"
				l.skipWhitespace()
				if l.pos < len(l.input) && isIdentStart(l.input[l.pos]) {
					nextIdent := l.peekIdent()
					if strings.ToLower(nextIdent) == "in" {
						l.readIdent() // consume "in"
						tokens = append(tokens, Token{Type: TokenNotIn, Literal: "not in", Position: startPos})
						continue
					} else if strings.ToLower(nextIdent) == "exists" {
						l.readIdent() // consume "exists"
						tokens = append(tokens, Token{Type: TokenNotExists, Literal: "not exists", Position: startPos})
						continue
					}
				}
				tokens = append(tokens, Token{Type: TokenNot, Literal: ident, Position: startPos})
			case "in":
				tokens = append(tokens, Token{Type: TokenIn, Literal: ident, Position: startPos})
			case "notin", "not_in":
				tokens = append(tokens, Token{Type: TokenNotIn, Literal: ident, Position: startPos})
			case "exists":
				tokens = append(tokens, Token{Type: TokenExists, Literal: ident, Position: startPos})
			case "not_exists":
				tokens = append(tokens, Token{Type: TokenNotExists, Literal: ident, Position: startPos})
			case "true", "false":
				tokens = append(tokens, Token{Type: TokenBool, Literal: lower, Position: startPos})
			default:
				tokens = append(tokens, Token{Type: TokenIdent, Literal: ident, Position: startPos})
			}
		default:
			return nil, fmt.Errorf("%w: unexpected character %q at position %d", ErrInvalidSelector, string(ch), startPos)
		}
	}

	tokens = append(tokens, Token{Type: TokenEOF, Literal: "", Position: len(l.input)})
	return tokens, nil
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *Lexer) peek() rune {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

func (l *Lexer) readString(quote rune) (string, error) {
	start := l.pos
	l.pos++ // Skip opening quote
	var b strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			l.pos++
			b.WriteRune(l.input[l.pos])
			l.pos++
			continue
		}
		if ch == quote {
			l.pos++ // Skip closing quote
			return b.String(), nil
		}
		b.WriteRune(ch)
		l.pos++
	}
	return "", fmt.Errorf("%w: unterminated string starting at position %d", ErrInvalidSelector, start)
}

func (l *Lexer) readNumber() string {
	start := l.pos
	if l.input[l.pos] == '-' {
		l.pos++
	}
	hasDot := false
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if unicode.IsDigit(ch) {
			l.pos++
		} else if ch == '.' && !hasDot && l.pos+1 < len(l.input) && unicode.IsDigit(l.input[l.pos+1]) {
			hasDot = true
			l.pos++
		} else {
			break
		}
	}
	return string(l.input[start:l.pos])
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' || r == '/'
}

func (l *Lexer) readIdent() string {
	start := l.pos
	for l.pos < len(l.input) && isIdentPart(l.input[l.pos]) {
		l.pos++
	}
	return string(l.input[start:l.pos])
}

func (l *Lexer) peekIdent() string {
	saved := l.pos
	for saved < len(l.input) && isIdentPart(l.input[saved]) {
		saved++
	}
	return string(l.input[l.pos:saved])
}

// Evaluation Context

// NodeEvaluationContext provides structured node attributes for deterministic selector evaluation.
type NodeEvaluationContext struct {
	NodeID              string
	Hostname            string
	Status              string
	OS                  string
	Platform            string
	Arch                string
	Version             string
	CPUCores            float64
	TotalMemoryBytes    float64
	MemoryGB            float64
	OwnerTeam           string
	Environment         string
	Region              string
	DataClassification  string
	CostCenter          string
	BusinessCriticality string
	Lifecycle           string
	GroupIDs            []string
	GroupPaths          []string
	Tags                map[string]string
	Metadata            map[string]string
	CustomProperties    map[string]string
}

// NewNodeEvaluationContext constructs an evaluation context from a node, ownership metadata, and group assignments.
func NewNodeEvaluationContext(node *model.FleetNode, ownership *model.NodeOwnershipMetadata, groupIDs []string, groupPaths []string) *NodeEvaluationContext {
	if node == nil {
		return &NodeEvaluationContext{}
	}

	ctx := &NodeEvaluationContext{
		NodeID:           node.Identity.NodeID,
		Hostname:         node.Identity.Hostname,
		Status:           string(node.Status),
		OS:               node.Identity.OS,
		Platform:         node.Identity.Platform,
		Arch:             node.Identity.Arch,
		Version:          node.Identity.Version,
		CPUCores:         float64(node.Identity.CPUCores),
		TotalMemoryBytes: float64(node.Identity.TotalMemory),
		MemoryGB:         float64(node.Identity.TotalMemory) / (1024 * 1024 * 1024),
		GroupIDs:         groupIDs,
		GroupPaths:       groupPaths,
		Tags:             node.Identity.Tags,
		Metadata:         node.Metadata,
	}

	if ctx.Tags == nil {
		ctx.Tags = make(map[string]string)
	}
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]string)
	}

	if ownership != nil {
		ctx.OwnerTeam = ownership.OwnerTeam
		ctx.Environment = ownership.Environment
		ctx.Region = ownership.Region
		ctx.DataClassification = ownership.DataClassification
		ctx.CostCenter = ownership.CostCenter
		ctx.BusinessCriticality = string(ownership.BusinessCriticality)
		ctx.Lifecycle = string(ownership.Lifecycle)
		ctx.CustomProperties = ownership.CustomProperties
	}

	// Fallback to metadata keys if ownership is unset
	if ctx.Environment == "" {
		if env, ok := ctx.Metadata[model.MetaKeyEnvironment]; ok {
			ctx.Environment = env
		} else if env, ok := ctx.Metadata["env"]; ok {
			ctx.Environment = env
		}
	}
	if ctx.Region == "" {
		if reg, ok := ctx.Metadata[model.MetaKeyRegion]; ok {
			ctx.Region = reg
		} else if reg, ok := ctx.Metadata["region"]; ok {
			ctx.Region = reg
		}
	}
	if ctx.OwnerTeam == "" {
		if team, ok := ctx.Metadata[model.MetaKeyOwnerTeam]; ok {
			ctx.OwnerTeam = team
		}
	}
	if ctx.BusinessCriticality == "" {
		if crit, ok := ctx.Metadata[model.MetaKeyCriticality]; ok {
			ctx.BusinessCriticality = crit
		}
	}
	if ctx.Lifecycle == "" {
		if lc, ok := ctx.Metadata[model.MetaKeyLifecycle]; ok {
			ctx.Lifecycle = lc
		}
	}

	return ctx
}

// GetProperty resolves a selector variable name into its value.
func (c *NodeEvaluationContext) GetProperty(field string) (any, bool) {
	if c == nil {
		return nil, false
	}

	lower := strings.ToLower(field)

	// Built-in attributes
	switch lower {
	case "node_id", "id":
		return c.NodeID, c.NodeID != ""
	case "hostname", "host":
		return c.Hostname, c.Hostname != ""
	case "status":
		return c.Status, c.Status != ""
	case "os":
		return c.OS, c.OS != ""
	case "platform":
		return c.Platform, c.Platform != ""
	case "arch":
		return c.Arch, c.Arch != ""
	case "version":
		return c.Version, c.Version != ""
	case "cpu_cores", "cores":
		return c.CPUCores, true
	case "total_memory", "total_memory_bytes", "memory_bytes":
		return c.TotalMemoryBytes, true
	case "memory_gb":
		return c.MemoryGB, true
	case "owner_team", "team":
		return c.OwnerTeam, c.OwnerTeam != ""
	case "environment", "env":
		return c.Environment, c.Environment != ""
	case "region":
		return c.Region, c.Region != ""
	case "data_classification", "classification":
		return c.DataClassification, c.DataClassification != ""
	case "cost_center":
		return c.CostCenter, c.CostCenter != ""
	case "business_criticality", "criticality":
		return c.BusinessCriticality, c.BusinessCriticality != ""
	case "lifecycle":
		return c.Lifecycle, c.Lifecycle != ""
	case "group_id", "group_ids":
		return c.GroupIDs, len(c.GroupIDs) > 0
	case "group", "group_path", "group_paths":
		return c.GroupPaths, len(c.GroupPaths) > 0
	}

	// Tags prefix: tags.key
	if strings.HasPrefix(lower, "tags.") {
		key := field[5:]
		if val, ok := c.Tags[key]; ok {
			return val, true
		}
		return nil, false
	}

	// Metadata prefix: metadata.key
	if strings.HasPrefix(lower, "metadata.") {
		key := field[9:]
		if val, ok := c.Metadata[key]; ok {
			return val, true
		}
		return nil, false
	}

	// Custom properties prefix: custom.key
	if strings.HasPrefix(lower, "custom.") {
		key := field[7:]
		if val, ok := c.CustomProperties[key]; ok {
			return val, true
		}
		return nil, false
	}

	// Fallback checks
	if val, ok := c.Metadata[field]; ok {
		return val, true
	}
	if val, ok := c.Tags[field]; ok {
		return val, true
	}
	if val, ok := c.CustomProperties[field]; ok {
		return val, true
	}

	return nil, false
}

// AST Nodes

// Expression is the common interface implemented by all AST nodes.
type Expression interface {
	Evaluate(ctx *NodeEvaluationContext) (bool, error)
	String() string
}

// OrExpression represents a logical OR between expressions.
type OrExpression struct {
	Left  Expression
	Right Expression
}

func (e *OrExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	left, err := e.Left.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	if left {
		return true, nil // Short circuit
	}
	return e.Right.Evaluate(ctx)
}

func (e *OrExpression) String() string {
	return fmt.Sprintf("(%s OR %s)", e.Left, e.Right)
}

// AndExpression represents a logical AND between expressions.
type AndExpression struct {
	Left  Expression
	Right Expression
}

func (e *AndExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	left, err := e.Left.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	if !left {
		return false, nil // Short circuit
	}
	return e.Right.Evaluate(ctx)
}

func (e *AndExpression) String() string {
	return fmt.Sprintf("(%s AND %s)", e.Left, e.Right)
}

// NotExpression represents a logical NOT on an expression.
type NotExpression struct {
	Expr Expression
}

func (e *NotExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	val, err := e.Expr.Evaluate(ctx)
	if err != nil {
		return false, err
	}
	return !val, nil
}

func (e *NotExpression) String() string {
	return fmt.Sprintf("NOT (%s)", e.Expr)
}

// ExistsExpression checks if a field exists and has a non-empty value.
type ExistsExpression struct {
	Field     string
	NotExists bool
}

func (e *ExistsExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	val, exists := ctx.GetProperty(e.Field)
	if !exists || val == nil {
		return e.NotExists, nil
	}

	// String empty check
	if s, ok := val.(string); ok && s == "" {
		return e.NotExists, nil
	}
	// Slice empty check
	if sl, ok := val.([]string); ok && len(sl) == 0 {
		return e.NotExists, nil
	}

	if e.NotExists {
		return false, nil
	}
	return true, nil
}

func (e *ExistsExpression) String() string {
	if e.NotExists {
		return fmt.Sprintf("NOT EXISTS %s", e.Field)
	}
	return fmt.Sprintf("EXISTS %s", e.Field)
}

// InExpression checks whether a field's value is (or intersects) a list of target values.
type InExpression struct {
	Field  string
	Values []string
	NotIn  bool
}

func (e *InExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	val, exists := ctx.GetProperty(e.Field)
	if !exists || val == nil {
		if e.NotIn {
			return true, nil
		}
		return false, nil
	}

	matchFound := false

	switch v := val.(type) {
	case string:
		for _, target := range e.Values {
			if v == target {
				matchFound = true
				break
			}
		}
	case []string:
		// Check if any element of slice matches target list
		for _, item := range v {
			for _, target := range e.Values {
				if item == target {
					matchFound = true
					break
				}
			}
			if matchFound {
				break
			}
		}
	case float64:
		for _, target := range e.Values {
			if num, err := strconv.ParseFloat(target, 64); err == nil && v == num {
				matchFound = true
				break
			}
		}
	default:
		s := fmt.Sprintf("%v", v)
		for _, target := range e.Values {
			if s == target {
				matchFound = true
				break
			}
		}
	}

	if e.NotIn {
		return !matchFound, nil
	}
	return matchFound, nil
}

func (e *InExpression) String() string {
	op := "IN"
	if e.NotIn {
		op = "NOT IN"
	}
	return fmt.Sprintf("%s %s [%s]", e.Field, op, strings.Join(e.Values, ", "))
}

// ComparisonExpression represents binary comparisons (==, !=, >, >=, <, <=).
type ComparisonExpression struct {
	Field    string
	Operator TokenType
	Value    string
}

func (e *ComparisonExpression) Evaluate(ctx *NodeEvaluationContext) (bool, error) {
	val, exists := ctx.GetProperty(e.Field)

	// Handle non-existent fields
	if !exists || val == nil {
		if e.Operator == TokenNeq {
			return true, nil
		}
		return false, nil
	}

	// Slice matching (e.g. group_id == "grp-prod")
	if sl, ok := val.([]string); ok {
		contains := false
		for _, s := range sl {
			if s == e.Value {
				contains = true
				break
			}
		}
		if e.Operator == TokenEq {
			return contains, nil
		}
		if e.Operator == TokenNeq {
			return !contains, nil
		}
		return false, fmt.Errorf("%w: cannot perform relational comparison on list property %q", ErrInvalidSelector, e.Field)
	}

	// Numeric comparisons
	if targetNum, err := strconv.ParseFloat(e.Value, 64); err == nil {
		var actualNum float64
		var isNum bool

		switch n := val.(type) {
		case float64:
			actualNum = n
			isNum = true
		case int:
			actualNum = float64(n)
			isNum = true
		case uint64:
			actualNum = float64(n)
			isNum = true
		case string:
			if parsed, parseErr := strconv.ParseFloat(n, 64); parseErr == nil {
				actualNum = parsed
				isNum = true
			}
		}

		if isNum {
			switch e.Operator {
			case TokenEq:
				return actualNum == targetNum, nil
			case TokenNeq:
				return actualNum != targetNum, nil
			case TokenGt:
				return actualNum > targetNum, nil
			case TokenGte:
				return actualNum >= targetNum, nil
			case TokenLt:
				return actualNum < targetNum, nil
			case TokenLte:
				return actualNum <= targetNum, nil
			}
		}
	}

	// Boolean comparisons
	if targetBool, err := strconv.ParseBool(e.Value); err == nil {
		var actualBool bool
		var isBool bool
		switch b := val.(type) {
		case bool:
			actualBool = b
			isBool = true
		case string:
			if parsed, parseErr := strconv.ParseBool(b); parseErr == nil {
				actualBool = parsed
				isBool = true
			}
		}

		if isBool {
			if e.Operator == TokenEq {
				return actualBool == targetBool, nil
			}
			if e.Operator == TokenNeq {
				return actualBool != targetBool, nil
			}
			return false, fmt.Errorf("%w: invalid operator %s for boolean", ErrInvalidSelector, e.OperatorString())
		}
	}

	// String comparison
	actualStr := fmt.Sprintf("%v", val)
	switch e.Operator {
	case TokenEq:
		return actualStr == e.Value, nil
	case TokenNeq:
		return actualStr != e.Value, nil
	case TokenGt:
		return actualStr > e.Value, nil
	case TokenGte:
		return actualStr >= e.Value, nil
	case TokenLt:
		return actualStr < e.Value, nil
	case TokenLte:
		return actualStr <= e.Value, nil
	default:
		return false, fmt.Errorf("%w: unsupported comparison operator", ErrInvalidSelector)
	}
}

func (e *ComparisonExpression) OperatorString() string {
	switch e.Operator {
	case TokenEq:
		return "=="
	case TokenNeq:
		return "!="
	case TokenGt:
		return ">"
	case TokenGte:
		return ">="
	case TokenLt:
		return "<"
	case TokenLte:
		return "<="
	default:
		return "??"
	}
}

func (e *ComparisonExpression) String() string {
	return fmt.Sprintf("%s %s %q", e.Field, e.OperatorString(), e.Value)
}

// Selector Parser

// Parser parses tokens into a safe AST.
type Parser struct {
	tokens []Token
	pos    int
	depth  int
}

// ParseSelector compiles a target selector string into an executable AST Expression.
func ParseSelector(query string) (Expression, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" || trimmed == "*" {
		return nil, nil // Empty or wildcard matches all
	}

	if len(trimmed) > MaxSelectorLength {
		return nil, fmt.Errorf("%w: selector length %d exceeds maximum %d", ErrSelectorTooLong, len(trimmed), MaxSelectorLength)
	}

	lexer := NewLexer(trimmed)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}

	p := &Parser{tokens: tokens, pos: 0, depth: 0}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}

	if p.current().Type != TokenEOF {
		return nil, fmt.Errorf("%w: unexpected token %q at position %d", ErrInvalidSelector, p.current().Literal, p.current().Position)
	}

	return expr, nil
}

func (p *Parser) current() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF}
}

func (p *Parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) match(types ...TokenType) bool {
	cur := p.current().Type
	for _, t := range types {
		if cur == t {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) parseOr() (Expression, error) {
	p.depth++
	if p.depth > MaxSelectorDepth {
		return nil, ErrSelectorMaxDepthExceeded
	}
	defer func() { p.depth-- }()

	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.match(TokenOr) {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &OrExpression{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseAnd() (Expression, error) {
	p.depth++
	if p.depth > MaxSelectorDepth {
		return nil, ErrSelectorMaxDepthExceeded
	}
	defer func() { p.depth-- }()

	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.match(TokenAnd) {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &AndExpression{Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseNot() (Expression, error) {
	p.depth++
	if p.depth > MaxSelectorDepth {
		return nil, ErrSelectorMaxDepthExceeded
	}
	defer func() { p.depth-- }()

	if p.match(TokenNot) {
		expr, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &NotExpression{Expr: expr}, nil
	}

	return p.parsePrimary()
}

func (p *Parser) parsePrimary() (Expression, error) {
	p.depth++
	if p.depth > MaxSelectorDepth {
		return nil, ErrSelectorMaxDepthExceeded
	}
	defer func() { p.depth-- }()

	cur := p.current()

	// Parenthesized sub-expression
	if p.match(TokenLParen) {
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match(TokenRParen) {
			return nil, fmt.Errorf("%w: missing closing parenthesis at position %d", ErrInvalidSelector, p.current().Position)
		}
		return expr, nil
	}

	// EXISTS / NOT_EXISTS <field>
	if p.match(TokenExists) {
		if p.current().Type != TokenIdent {
			return nil, fmt.Errorf("%w: expected identifier after EXISTS at position %d", ErrInvalidSelector, p.current().Position)
		}
		field := p.advance().Literal
		return &ExistsExpression{Field: field, NotExists: false}, nil
	}
	if p.match(TokenNotExists) {
		if p.current().Type != TokenIdent {
			return nil, fmt.Errorf("%w: expected identifier after NOT EXISTS at position %d", ErrInvalidSelector, p.current().Position)
		}
		field := p.advance().Literal
		return &ExistsExpression{Field: field, NotExists: true}, nil
	}

	// Ident comparisons: <ident> <op> <val> or <ident> IN [...]
	if p.current().Type == TokenIdent {
		field := p.advance().Literal

		// Check for IN / NOT IN
		if p.match(TokenIn) || p.match(TokenNotIn) {
			notIn := p.tokens[p.pos-1].Type == TokenNotIn
			if !p.match(TokenLBracket) {
				return nil, fmt.Errorf("%w: expected '[' after IN at position %d", ErrInvalidSelector, p.current().Position)
			}
			var values []string
			for p.current().Type != TokenRBracket && p.current().Type != TokenEOF {
				valTok := p.current()
				if valTok.Type == TokenString || valTok.Type == TokenNumber || valTok.Type == TokenIdent || valTok.Type == TokenBool {
					values = append(values, valTok.Literal)
					p.advance()
				} else {
					return nil, fmt.Errorf("%w: unexpected token in list at position %d", ErrInvalidSelector, valTok.Position)
				}

				if p.match(TokenComma) {
					continue
				} else if p.current().Type == TokenRBracket {
					break
				} else {
					return nil, fmt.Errorf("%w: expected comma or ']' at position %d", ErrInvalidSelector, p.current().Position)
				}
			}
			if !p.match(TokenRBracket) {
				return nil, fmt.Errorf("%w: missing closing ']' at position %d", ErrInvalidSelector, p.current().Position)
			}
			return &InExpression{Field: field, Values: values, NotIn: notIn}, nil
		}

		// Check for comparison operators
		opTok := p.current()
		if p.match(TokenEq, TokenNeq, TokenGt, TokenGte, TokenLt, TokenLte) {
			valTok := p.current()
			if valTok.Type == TokenString || valTok.Type == TokenNumber || valTok.Type == TokenIdent || valTok.Type == TokenBool {
				p.advance()
				return &ComparisonExpression{Field: field, Operator: opTok.Type, Value: valTok.Literal}, nil
			}
			return nil, fmt.Errorf("%w: expected literal value after operator at position %d", ErrInvalidSelector, valTok.Position)
		}

		return nil, fmt.Errorf("%w: expected operator after identifier %q at position %d", ErrInvalidSelector, field, p.current().Position)
	}

	return nil, fmt.Errorf("%w: unexpected token %q at position %d", ErrInvalidSelector, cur.Literal, cur.Position)
}

// MatchesNode is a high-level helper to test whether a selector query matches a node.
func MatchesNode(selector string, node *model.FleetNode, ownership *model.NodeOwnershipMetadata, groupIDs []string, groupPaths []string) (bool, error) {
	expr, err := ParseSelector(selector)
	if err != nil {
		return false, err
	}
	if expr == nil {
		return true, nil // Empty or nil selector matches everything
	}

	ctx := NewNodeEvaluationContext(node, ownership, groupIDs, groupPaths)
	return expr.Evaluate(ctx)
}
