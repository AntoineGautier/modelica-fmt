// Copyright (c) 2020, Alliance for Sustainable Energy, LLC.
// All rights reserved.

package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/urbanopt/modelica-fmt/thirdparty/parser"
)

type Config struct {
	maxLineLength int
	emptyLines    bool
}

const (
	// indent
	spaceIndent = "  "
)

// insertIndentBefore returns true if the rule should be on a new line and indented
func (l *modelicaListener) insertIndentBefore(rule antlr.ParserRuleContext) bool {
	switch rule.(type) {
	case
		parser.IElementContext,
		parser.IEquationsContext,
		parser.IAlgorithm_statementsContext,
		parser.IControl_structure_bodyContext,
		parser.IAnnotationContext,
		parser.IEnumeration_literalContext,
		parser.ICondition_attributeContext,
		parser.IExpression_listContext,
		parser.IConstraining_clauseContext,
		parser.IExternal_function_call_argumentContext:
		return true
	case parser.IString_commentContext:
		return 0 == l.inAnnotation
	case
		parser.IArgumentContext,
		parser.INamed_argumentContext:
		if 0 == l.inAnnotation || 0 < l.inModelAnnotation {
			return true
		} else if 0 < l.inAnnotation {
			// BUG: despite within insertIndentBefore, the following rule yields no incremental indentation
			// vendor annotations starting with "__" even have missing indentation
			matched, _ := regexp.MatchString(
				"choice|^enable|iconTransformation|Placement|Dialog|Evaluate|^__",
				rule.GetText())
			return (matched) && l.previousTokenText != "("
		} else {
			return false
		}
	case parser.IExpressionContext:
		if len(l.modelAnnotationVectorStack) == 0 {
			return false
		}

		// handle expression which is an element of a vector (array_arguments) and within model annotation
		arrayArgumentsNode, ok := rule.GetParent().(*parser.Array_argumentsContext)
		if !ok {
			return false
		}

		// check if the vector is the same as the one on top of our stack
		thisVectorInterval := arrayArgumentsNode.GetParent().(*parser.VectorContext).GetSourceInterval()
		stackVectorInterval := l.modelAnnotationVectorStack[len(l.modelAnnotationVectorStack)-1].GetSourceInterval()
		if thisVectorInterval.Start == stackVectorInterval.Start && thisVectorInterval.Stop == stackVectorInterval.Stop {
			return true
		}
		return false
	// case parser.IFunction_argumentContext:
	// 	return 0 == l.inNamedArgument && 0 == l.inVector && (0 == l.inAnnotation || 0 < l.inModelAnnotation)
	default:
		return false
	}
}

// insertSpaceBeforeToken returns true if a space should be inserted before the current token
func (l *modelicaListener) insertSpaceBeforeToken(currentTokenText, previousTokenText string) bool {
	switch currentTokenText {
	case "(":
		// add a space before opening parens for the following exceptions
		matched, _ := regexp.MatchString(
			"\\bannotation\\b|\\bif\\b|\\bthen\\b|\\band\\b|\\bor\\b|\\belse\\b|\\belseif\\b",
			previousTokenText)
		if (matched) {
			return true
		}
		fallthrough
	default:
		return 0 == l.inAnnotation &&
			   !tokenInGroup(previousTokenText, noSpaceAfterTokens, false) &&
			   !tokenInGroup(currentTokenText, noSpaceBeforeTokens, false) ||
			   0 < l.inAnnotation &&
			   !tokenInGroup(previousTokenText, noSpaceAroundTokensInAnnotation, false) &&
			   !tokenInGroup(currentTokenText, noSpaceAroundTokensInAnnotation, false)
	}
}

// the following rule does not change the results: we get rid of it

// insertNewlineBefore returns true if the rule should be on a new line
// func (l *modelicaListener) insertNewlineBefore(rule antlr.ParserRuleContext) bool {
// 	switch rule.(type) {
// 	case
// 		parser.ICompositionContext,
// 		parser.IEnumeration_literalContext,
// 		parser.IEquationsContext,
// 		parser.ICondition_attributeContext:
// 		return true
// 	default:
// 		return false
// 	}
// }

var (
	// tokens which should *generally* not have a space after them
	// this can be overridden in the insertSpace function
	noSpaceAfterTokens = []string{
		"(",
		"=",
		// "==",
		// "<>",
		".",
		"[",
		"{",
		// "-", "+", "^", "*", "/",
		";",
		// ",",
		":", // array range constructor
	}

	// tokens which should *generally* not have a space before them
	// this can be overridden in the insertSpace function
	noSpaceBeforeTokens = []string{
		"(", ")",
		"[", "]",
		"}",
		";",
		"=",
		// "==",
		// "<>",
		",",
		".",
		// "-", "+", "^", "*", "/",
		":", // array range constructor
	}

	noSpaceAroundTokensInAnnotation = []string{
		"(", ")",
		"[", "]",
		"{", "}",
		";",
		"=",
		"==",
		"<>",
		",",
		".",
		"-", "+", "^", "*", "/",
		":",
	}

	// following rules only applied to limit line length
	// not applied within element annotations
	allowBreakAfterTokens = []string{
		";",
		"+",
		"=",
		"==",
		"<>",
	}

	// following rules only applied to limit line length
	// applied within element annotations: we only allow breaking annotations for ad hoc keywords
	// search for the following patterns is done with regexp.MatchString(): use "\\bword\\b" to search exactly "word"
	allowBreakBeforeTokens = []string{
		"\".*\"",
		"\\bcolor\\b",
		"\\bextent\\b",
		"\\bgroup\\b",
		"\\bif\\b",
		"\\bthen\\b",
		"\\belse\\b",
		"\\belseif\\b",
		"\\band\\b",
		"\\bor\\b",
		"\\bhorizontalAlignment\\b",
		"\\bLine\\b",
		"\\bPolygon\\b",
		"\\bRectangle\\b",
		"\\bEllipse\\b",
		"\\bText\\b",
		"\\bBitmap\\b",
		"\\borigin\\b",
		"\\bpoints\\b",
		"\\brotation\\b",
		"\\btransformation\\b",
		"\\bvisible\\b",
	}
)

// tokenInGroup returns true if a token is in a given list
func tokenInGroup(token string, group []string, useRegex bool) bool {
	for _, other := range group {
		if useRegex {
			matched, _ := regexp.MatchString(other, token)
			if (matched) {
				return true
			}
		} else {
			if token == other {
				return true
			}
		}
	}
	return false
}

type indent int

const (
	renderIndent indent = iota
	ignoreIndent
)

// modelicaListener is used to format the parse tree
type modelicaListener struct {
	*parser.BaseModelicaListener               // parser
	writer                       *bufio.Writer // writing destination
	indentationStack             []indent      // a stack used for tracking rendered and ignored indentations
	onNewLine                    bool          // true when write position succeeds a newline character
	withinOnCurrentLine          bool          // true when `within` statement is found on the current line
	insideBracket                bool          // true when inside brackets (i.e. `[]`)
	lineIndentIncreased          bool          // true when the indentation level has already been increased for a line
	previousTokenText            string        // text of previous token
	previousTokenIdx             int           // index of previous token
	commentTokens                []antlr.Token // stores comments to insert while writing
	maxLineLength                int           // configuration for num charaters per line
	currentLineLength            int           // length of the line up to the writing position

	// modelAnnotationVectorStack is a stack which stores `vector` contexts,
	// which is used for conditionally indenting vector children
	// Specifically, a vector inside of a model annotation will have indented elements
	// if that vector has one or more elements which are function calls, class modifications or similar
	// (ie not if all elements are numbers, more vectors, etc)
	//
	// The last element of the slice is the first `vector` context ancestor whose contents
	// must be indented on new lines
	// For example, we would like model annotations to look like this:
	// annotation (
	// 	Abc(
	// 		paramA={
	// 			SomeIdentifier(
	// 				1,
	// 				2),
	//			123}))
	//
	// However, due to existing rules, we would end up with something like this
	// annotation (
	// 	Abc(
	// 		paramA={SomeIdentifier(
	// 			1,
	// 			2), 123}))
	//
	// Thus by pushing/popping vectors we can check if an expression in a vector
	// should be indented or not by checking if the top of the stack is its ancestor
	modelAnnotationVectorStack []antlr.RuleContext

	// NOTE: consider refactoring this simple approach for context awareness with
	// a set.
	// It should probably be map[string]int for rule name and current count (rules can be recursive, ie inside the same rule multiple times)
	inAnnotation      int  // counts number of current or ancestor contexts that are annotation rule
	inModelAnnotation int  // counts number of current or ancestor contexts that are model annotation rule
	inNamedArgument   int  // counts number of current or ancestor contexts that are named argument
	inVector          int  // counts number of current or ancestor contexts that are vector
	inLastSemicolon   bool // true if the listener is handling the last_semicolon rule

	// Other config
	config Config
}

func newListener(out io.Writer, commentTokens []antlr.Token, config Config) *modelicaListener {
	return &modelicaListener{
		BaseModelicaListener: &parser.BaseModelicaListener{},
		writer:               bufio.NewWriter(out),
		onNewLine:            true,
		withinOnCurrentLine:  false,
		insideBracket:        false,
		lineIndentIncreased:  false,
		inLastSemicolon:      false,
		inAnnotation:         0,
		inModelAnnotation:    0,
		inVector:             0,
		inNamedArgument:      0,
		previousTokenText:    "",
		previousTokenIdx:     -1,
		commentTokens:        commentTokens,
		currentLineLength:    0,
		config:               config,
	}
}

func (l *modelicaListener) close() {
	err := l.writer.Flush()
	if err != nil {
		panic(err)
	}
}

// indentation returns the writer's current number of *rendered* indentations
func (l *modelicaListener) indentation() int {
	nRenderIndents := 0
	for _, indentType := range l.indentationStack {
		if indentType == renderIndent {
			nRenderIndents++
		}
	}

	return nRenderIndents
}

// maybeIndent should be called when the writer's indentation is to be increased
func (l *modelicaListener) maybeIndent() {
	// Only increase indentation if it hasn't been changed already, otherwise ignore it
	// NOTE: This means that there can be at most 1 increase in indentation per line
	// This is a bit of a hack to avoid having an overindented line, occurring when
	// multiple rules want to be indented and we want it to be indented only once

	if !l.lineIndentIncreased {
		l.indentationStack = append(l.indentationStack, renderIndent)

		// WARNING: this is coupled with writeNewline, which should reset
		// lineIndentIncreased to false
		l.lineIndentIncreased = true
	} else {
		l.indentationStack = append(l.indentationStack, ignoreIndent)
	}
}

// maybeDedent should be called when the writer's indentation is to be decreased
func (l *modelicaListener) maybeDedent() {
	if len(l.indentationStack) > 0 {
		l.indentationStack = l.indentationStack[:len(l.indentationStack)-1]
	}
}

// writeString writes a string to the listener's output
// It should serve as the main entrypoint to writing to the output
func (l *modelicaListener) writeString(str string) {
	originalSpacePrefix := l.getSpaceBefore(str, true)
	charsOnFirstLine := len(originalSpacePrefix)
	firstNewlineIndex := strings.Index(str, "\n")
	if firstNewlineIndex < 0 {
		charsOnFirstLine += len(str)
	} else {
		charsOnFirstLine += firstNewlineIndex
	}

	// break the line if writing this string would make it too long and the previous token is breakable
	var actualSpacePrefix string
	if l.config.maxLineLength > 0 &&
	   l.currentLineLength+charsOnFirstLine > l.config.maxLineLength &&
	   !l.onNewLine &&
	   (l.inAnnotation == 0 && tokenInGroup(l.previousTokenText, allowBreakAfterTokens, false) ||
	    tokenInGroup(str, allowBreakBeforeTokens, true)){

		l.writeNewline()
		l.maybeIndent()
		actualSpacePrefix = l.getSpaceBefore(str, false)
		l.writer.WriteString(actualSpacePrefix + str)
		l.maybeDedent()
	} else {
		actualSpacePrefix = l.getSpaceBefore(str, false)
		l.writer.WriteString(actualSpacePrefix + str)
	}

	lastNewlineIndex := strings.LastIndex(str, "\n")
	var charsOnLastLine int
	if lastNewlineIndex < 0 {
		charsOnLastLine = len(actualSpacePrefix) + len(str)
	} else {
		// since there was a newline, no need to add the space prefix to the count
		charsOnLastLine = len(str) - (lastNewlineIndex + 1)
	}
	l.currentLineLength += charsOnLastLine
}

func (l *modelicaListener) writeNewline() {
	// explicitly not using l.writeString here b/c it's not necessary and I think we could end up in infinite recursion (though really unlikely)
	l.writer.WriteString("\n")
	l.onNewLine = true
	l.currentLineLength = 0

	// WARNING: this is coupled with maybeIndent, which uses this state
	l.lineIndentIncreased = false
}

func (l *modelicaListener) writeComment(comment antlr.Token) {
	l.writeString(comment.GetText())
	if comment.GetTokenType() == parser.ModelicaLexerLINE_COMMENT {
		l.writeNewline()
	}
}

// getSpaceBefore returns whitespace that should prefix the string. Note that this can modify the listener state
// If dryRun is true, the function will NOT modify the listener state (useful for predicting what the space will be)
func (l *modelicaListener) getSpaceBefore(str string, dryRun bool) string {
	if l.onNewLine {
		if !dryRun {
			l.onNewLine = false
		}

		// insert indentation
		if l.indentation() > 0 {
			indentation := l.indentation()
			return strings.Repeat(spaceIndent, indentation)
		}
	} else if l.insertSpaceBeforeToken(str, l.previousTokenText) {
		// insert a space
		return " "
	}
	return ""
}

// insertBlankLine returns true if an empty line should be inserted
// Used when visiting a terminal semicolon (ie ';')
func (l *modelicaListener) insertBlankLine() bool {
	if !l.config.emptyLines {
		return false
	}

	// if at the end of the file (ie the last semicolon) only insert an extra
	// line if there are comments remaining which will be appended at the end of
	// the file
	if l.inLastSemicolon {
		return len(l.commentTokens) > 0
	}

	// only insert a blank line if there's no `within` on current line,
	// and we're outside of brackets
	return !l.withinOnCurrentLine && !l.insideBracket
}

func (l *modelicaListener) VisitTerminal(node antlr.TerminalNode) {
	// if there's a comment that should go before this node, insert it first
	tokenIdx := node.GetSymbol().GetTokenIndex()
	for len(l.commentTokens) > 0 && tokenIdx > l.commentTokens[0].GetTokenIndex() && l.commentTokens[0].GetTokenIndex() > l.previousTokenIdx {
		commentToken := l.commentTokens[0]
		l.commentTokens = l.commentTokens[1:]
		l.writeComment(commentToken)
	}

	l.writeString(node.GetText())

	if l.previousTokenText == "within" {
		l.withinOnCurrentLine = true
	}

	if l.previousTokenText == "[" {
		l.insideBracket = true
	} else if l.previousTokenText == "]" {
		l.insideBracket = false
	}

	if node.GetText() == ";" {
		l.writeNewline()

		if l.insertBlankLine() {
			l.writeNewline()
		} else {
			l.withinOnCurrentLine = false
		}
	}

	l.previousTokenText = node.GetText()
	l.previousTokenIdx = node.GetSymbol().GetTokenIndex()
}

func (l *modelicaListener) EnterEveryRule(node antlr.ParserRuleContext) {
	// if l.insertNewlineBefore(node) && !l.onNewLine {
	// 	l.writeNewline()
	// }

	if l.insertIndentBefore(node) {
		if !l.onNewLine {
			l.writeNewline()
		}
		l.maybeIndent()
	}
}

func (l *modelicaListener) ExitEveryRule(node antlr.ParserRuleContext) {
	if l.insertIndentBefore(node) {
		l.maybeDedent()
	}
}

func (l *modelicaListener) EnterAnnotation(node *parser.AnnotationContext) {
	l.inAnnotation++
}

func (l *modelicaListener) ExitAnnotation(node *parser.AnnotationContext) {
	l.inAnnotation--
}

func (l *modelicaListener) EnterModel_annotation(node *parser.Model_annotationContext) {
	l.inModelAnnotation++
}

func (l *modelicaListener) ExitModel_annotation(node *parser.Model_annotationContext) {
	l.inModelAnnotation--
}

func (l *modelicaListener) EnterVector(node *parser.VectorContext) {
	l.inVector++
	if l.inModelAnnotation > 0 {
		// if this array uses an iterator for construction it gets no special treatment
		if _, ok := node.GetChild(0).(parser.Array_iterator_constructorContext); ok {
			return
		}

		// check if there is an element of this vector which would require indentation
		for _, child := range node.Array_arguments().GetChildren() {
			expressionNode, ok := child.(*parser.ExpressionContext)
			if !ok {
				continue
			}
			startToken := expressionNode.GetStart()
			if startToken.GetTokenType() == parser.ModelicaLexerIDENT {
				l.modelAnnotationVectorStack = append(l.modelAnnotationVectorStack, node)
				break
			}
		}
	}
}

func (l *modelicaListener) ExitVector(node *parser.VectorContext) {
	l.inVector--
	if len(l.modelAnnotationVectorStack) > 0 {
		annotationVectorInterval := l.modelAnnotationVectorStack[len(l.modelAnnotationVectorStack)-1].GetSourceInterval()
		thisVectorInterval := node.GetSourceInterval()
		if annotationVectorInterval.Start == thisVectorInterval.Start && annotationVectorInterval.Stop == thisVectorInterval.Stop {
			l.modelAnnotationVectorStack = l.modelAnnotationVectorStack[:len(l.modelAnnotationVectorStack)-1]
		}
	}
}

func (l *modelicaListener) EnterNamed_argument(node *parser.Named_argumentContext) {
	l.inNamedArgument++
}

func (l *modelicaListener) ExitNamed_argument(node *parser.Named_argumentContext) {
	l.inNamedArgument--
}

func (l *modelicaListener) EnterLast_semicolon(node *parser.Last_semicolonContext) {
	l.inLastSemicolon = true
}

func (l *modelicaListener) ExitLast_semicolon(node *parser.Last_semicolonContext) {
	l.inLastSemicolon = false
}

// commentCollector is a wrapper around the default lexer which collects comment
// tokens for later use
type commentCollector struct {
	antlr.TokenSource
	commentTokens []antlr.Token
}

func newCommentCollector(source antlr.TokenSource) commentCollector {
	return commentCollector{
		source,
		[]antlr.Token{},
	}
}

// NextToken returns the next token from the source
func (c *commentCollector) NextToken() antlr.Token {
	token := c.TokenSource.NextToken()

	tokenType := token.GetTokenType()
	if tokenType == parser.ModelicaLexerCOMMENT || tokenType == parser.ModelicaLexerLINE_COMMENT {
		c.commentTokens = append(c.commentTokens, token)
	}

	return token
}

// processFile formats a file
func processFile(filename string, out io.Writer, config Config) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	text := string(content)
	inputStream := antlr.NewInputStream(text)
	lexer := parser.NewModelicaLexer(inputStream)

	// wrap the default lexer to collect comments and set it as the stream's source
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	tokenSource := newCommentCollector(lexer)
	stream.SetTokenSource(&tokenSource)

	p := parser.NewModelicaParser(stream)
	sd := p.Stored_definition()

	listener := newListener(out, tokenSource.commentTokens, config)
	defer listener.close()

	antlr.ParseTreeWalkerDefault.Walk(listener, sd)
	// add any remaining comments and handle newline at end of file
	for _, comment := range listener.commentTokens {
		listener.writeComment(comment)
	}
	if !listener.onNewLine {
		listener.writeNewline()
	}

	return nil
}
