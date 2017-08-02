package nexus

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fredericlemoine/goalign/align"
	treeio "github.com/fredericlemoine/gotree/io"
	"github.com/fredericlemoine/gotree/io/newick"
)

// Parser represents a parser.
type Parser struct {
	s   *Scanner
	buf struct {
		tok Token  // last read token
		lit string // last read literal
		n   int    // buffer size (max=1)
	}
}

// NewParser returns a new instance of Parser.
func NewParser(r io.Reader) *Parser {
	return &Parser{s: NewScanner(r)}
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that instead.
func (p *Parser) scan() (tok Token, lit string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		p.buf.n = 0
		return p.buf.tok, p.buf.lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit = p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buf.tok, p.buf.lit = tok, lit

	return
}

// unscan pushes the previously read token back onto the buffer.
func (p *Parser) unscan() { p.buf.n = 1 }

// scanIgnoreWhitespace scans the next non-whitespace token.
func (p *Parser) scanIgnoreWhitespace() (tok Token, lit string) {
	tok, lit = p.scan()
	if tok == WS {
		tok, lit = p.scan()
	}
	return
}

// Parses a Newick String.
func (p *Parser) Parse() (*Nexus, error) {
	var nchar, ntax int64
	datatype := "dna"
	missing := '*'
	gap := '-'
	var taxlabels map[string]bool = nil
	var names, sequences, treestrings, treenames []string
	nexus := NewNexus()

	// First token should be a "NEXUS" token.
	tok, lit := p.scanIgnoreWhitespace()
	if tok != NEXUS {
		return nil, fmt.Errorf("found %q, expected #NEXUS", lit)
	}

	// Now we can parse the remaining of the file
	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tok == ILLEGAL {
			return nil, fmt.Errorf("found illegal token %q", lit)
		}
		if tok == EOF {
			break
		}
		if tok == ENDOFLINE {
			continue
		}
		// Beginning of a block
		if tok == BEGIN {
			// Next token should be the name of the block
			tok2, lit2 := p.scanIgnoreWhitespace()
			// Then a ;
			tok3, _ := p.scanIgnoreWhitespace()
			if tok3 != ENDOFCOMMAND {
				return nil, fmt.Errorf("found %q, expected ;", lit)
			}
			var err error
			switch tok2 {
			case TAXA:
				// TAXA BLOCK
				taxlabels, err = p.parseTaxa()
			case TREES:
				// TREES BLOCK
				treenames, treestrings, err = p.parseTrees()
			case DATA:
				// DATA/CHARACTERS BLOCK
				names, sequences, nchar, ntax, datatype, missing, gap, err = p.parseData()
			default:
				// If an unsupported block is seen, we just skip it
				treeio.LogWarning(fmt.Errorf("Unsupported block %q, skipping", lit2))
				err = p.parseUnsupportedBlock()
			}

			if err != nil {
				return nil, err
			}
		}
	}

	if gap != '-' || missing != '*' {
		return nil, fmt.Errorf("We only accept - gaps (not %c) && * missing (not %c) so far", gap, missing)
	}

	// We initialize alignment structure using goalign structure
	if names != nil && sequences != nil {
		al := align.NewAlign(align.AlphabetFromString(datatype))
		if al.Alphabet() == align.UNKNOWN {
			return nil, fmt.Errorf("Unknown datatype: %q", datatype)
		}
		if len(names) != int(ntax) && ntax != -1 {
			return nil, fmt.Errorf("Number of taxa in alignment (%d)  does not correspond to definition %d", len(names), ntax)
		}
		for i, seq := range sequences {
			if len(seq) != int(nchar) && nchar != -1 {
				return nil, fmt.Errorf("Number of character in sequence #%d (%d) does not correspond to definition %d", i, len(seq), nchar)
			}
			if err := al.AddSequence(names[i], seq, ""); err != nil {
				return nil, err
			}
		}
		// We check that tax labels are the same as alignment sequence names
		if taxlabels != nil {
			var err error
			al.Iterate(func(name string, sequence string) {
				if _, ok := taxlabels[name]; !ok {
					err = fmt.Errorf("Sequence name %s in the alignment is not defined in the TAXLABELS block", name)
				}
			})
			if err != nil {
				return nil, err
			}
			if al.NbSequences() != len(taxlabels) {
				return nil, fmt.Errorf("Some taxa names defined in TAXLABELS are not present in the alignment")
			}
		}

		nexus.SetAlignment(al)
	}
	// We initialize tree structures using gotree structure
	if treenames != nil && treestrings != nil {
		for i, treestr := range treestrings {
			t, err := newick.NewParser(strings.NewReader(treestr + ";")).Parse()
			if err != nil {
				return nil, err
			}
			// We check that tax labels are the same as tree taxa
			if taxlabels != nil {
				tips := t.Tips()
				for _, tip := range tips {
					if _, ok := taxlabels[tip.Name()]; !ok {
						return nil, fmt.Errorf("Taxa name %s in the tree %d is not defined in the TAXLABELS block", i, tip.Name())
					}
				}
				if len(tips) != len(taxlabels) {
					return nil, fmt.Errorf("Some tax names defined in TAXLABELS are not present in the tree %d", i)
				}
			}
			nexus.AddTree(treenames[i], t)
		}
	}
	return nexus, nil
}

// Parse taxa block
func (p *Parser) parseTaxa() (map[string]bool, error) {
	taxlabels := make(map[string]bool)
	var err error
	stoptaxa := false
	for !stoptaxa {
		tok, lit := p.scanIgnoreWhitespace()
		switch tok {
		case ENDOFLINE:
			continue
		case ILLEGAL:
			err = fmt.Errorf("found illegal token %q", lit)
			stoptaxa = true
		case EOF:
			err = fmt.Errorf("End of file within a TAXA block (no END;)")
			stoptaxa = true
		case END:
			tok2, _ := p.scanIgnoreWhitespace()
			if tok2 != ENDOFCOMMAND {
				err = fmt.Errorf("End token without ;")
			}
			stoptaxa = true
		case TAXLABELS:
			stoplabels := false
			for !stoplabels {
				tok2, lit2 := p.scanIgnoreWhitespace()
				switch tok2 {
				case ENDOFCOMMAND:
					stoplabels = true
				case IDENT:
					taxlabels[lit2] = true
				default:
					err = fmt.Errorf("Unknown token %q in taxlabel list", lit2)
					stoplabels = true
				}
			}
			if err != nil {
				stoptaxa = true
			}
		default:
			err = p.parseUnsupportedCommand()
			treeio.LogWarning(fmt.Errorf("Unsupported command %q in block TAXA, skipping", lit))
			if err != nil {
				stoptaxa = true
			}
		}
	}
	return taxlabels, err
}

// Parse TREES block
func (p *Parser) parseTrees() (treenames, treestrings []string, err error) {
	treenames = make([]string, 0)
	treestrings = make([]string, 0)
	stoptrees := false
	for !stoptrees {
		tok, lit := p.scanIgnoreWhitespace()
		switch tok {
		case ENDOFLINE:
			continue
		case ILLEGAL:
			err = fmt.Errorf("found illegal token %q", lit)
			stoptrees = true
		case EOF:
			err = fmt.Errorf("End of file within a TREES block (no END;)")
			stoptrees = true
		case END:
			tok2, _ := p.scanIgnoreWhitespace()
			if tok2 != ENDOFCOMMAND {
				err = fmt.Errorf("End token without ;")
			}
			stoptrees = true
		case TREE:
			// A new tree is seen
			tok2, lit2 := p.scanIgnoreWhitespace()
			if tok2 != IDENT {
				err = fmt.Errorf("Expecting a tree name after TREE, got %q", lit2)
				stoptrees = true
			}
			tok3, lit3 := p.scanIgnoreWhitespace()
			if tok3 != EQUAL {
				err = fmt.Errorf("Expecting '=' after tree name, got %q", lit3)
				stoptrees = true
			}
			// We remove whitespaces in the tree string if any...
			tok4, lit4 := p.scanIgnoreWhitespace()
			tree := ""
			for tok4 != ENDOFCOMMAND {
				if tok4 != IDENT {
					err = fmt.Errorf("Expecting a tree after 'TREE name =', got  %q", lit4)
					stoptrees = true
				}
				tree += lit4
				tok4, lit4 = p.scanIgnoreWhitespace()
			}
			if tok4 != ENDOFCOMMAND {
				err = fmt.Errorf("Expecting ';' after 'TREE name = tree', got %q", lit4)
				stoptrees = true
			}
			treenames = append(treenames, lit2)
			treestrings = append(treestrings, tree)
		default:
			err = p.parseUnsupportedCommand()
			treeio.LogWarning(fmt.Errorf("Unsupported command %q in block TREES, skipping", lit))
			if err != nil {
				stoptrees = true
			}
		}
	}
	return
}

// DATA / Characters BLOCK
func (p *Parser) parseData() (names, sequences []string, nchar, ntax int64, datatype string, missing, gap rune, err error) {
	datatype = "dna"
	missing = '*'
	gap = '-'
	stopdata := false
	sequences = make([]string, 0)
	names = make([]string, 0)
	nchar = -1
	ntax = -1
	for !stopdata {
		tok, lit := p.scanIgnoreWhitespace()
		switch tok {
		case ENDOFLINE:
			break
		case ILLEGAL:
			err = fmt.Errorf("found illegal token %q", lit)
			stopdata = true
		case EOF:
			err = fmt.Errorf("End of file within a TAXA block (no END;)")
			stopdata = true
		case END:
			tok2, _ := p.scanIgnoreWhitespace()
			if tok2 != ENDOFCOMMAND {
				err = fmt.Errorf("End token without ;")
			}
			stopdata = true
		case DIMENSIONS:
			// Dimensions of the data: nchar , ntax
			stopdimensions := false
			for !stopdimensions {
				tok2, lit2 := p.scanIgnoreWhitespace()
				switch tok2 {
				case ENDOFCOMMAND:
					stopdimensions = true
				case NTAX:
					tok3, lit3 := p.scanIgnoreWhitespace()
					if tok3 != EQUAL {
						err = fmt.Errorf("Expecting '=' after NTAX, got %q", lit3)
						stopdimensions = true
					}
					tok4, lit4 := p.scanIgnoreWhitespace()
					if tok4 != NUMERIC {
						err = fmt.Errorf("Expecting Integer value after 'NTAX=', got %q", lit4)
						stopdimensions = true
					}
					ntax, err = strconv.ParseInt(lit4, 10, 64)
					if err != nil {
						stopdimensions = true
					}
				case NCHAR:
					tok3, lit3 := p.scanIgnoreWhitespace()
					if tok3 != EQUAL {
						err = fmt.Errorf("Expecting '=' after NTAX, got %q", lit3)
						stopdimensions = true
					}
					tok4, lit4 := p.scanIgnoreWhitespace()
					if tok4 != NUMERIC {
						err = fmt.Errorf("Expecting Integer value after 'NTAX=', got %q", lit4)
						stopdimensions = true
					}
					nchar, err = strconv.ParseInt(lit4, 10, 64)
					if err != nil {
						stopdimensions = true
					}
				default:
					if err = p.parseUnsupportedKey(lit2); err != nil {
						stopdimensions = true
					}
					treeio.LogWarning(fmt.Errorf("Unsupported key %q in %q command, skipping", lit2, lit))
				}
				if err != nil {
					stopdata = true
				}
			}
		case FORMAT:
			// Format of the data bock: datatype, missing, gap
			stopformat := false
			for !stopformat {
				tok2, lit2 := p.scanIgnoreWhitespace()

				switch tok2 {
				case ENDOFCOMMAND:
					stopformat = true
				case DATATYPE:
					tok3, lit3 := p.scanIgnoreWhitespace()
					if tok3 != EQUAL {
						err = fmt.Errorf("Expecting '=' after DATATYPE, got %q", lit3)
						stopformat = true
					}
					tok4, lit4 := p.scanIgnoreWhitespace()
					if tok4 == IDENT {
						datatype = lit4
					} else {
						err = fmt.Errorf("Expecting identifier after 'DATATYPE=', got %q", lit4)
						stopformat = true
					}
				case MISSING:
					tok3, lit3 := p.scanIgnoreWhitespace()
					if tok3 != EQUAL {
						err = fmt.Errorf("Expecting '=' after MISSING, got %q", lit3)
						stopformat = true
					}
					tok4, lit4 := p.scanIgnoreWhitespace()
					if tok4 != IDENT {
						err = fmt.Errorf("Expecting Integer value after 'MISSING=', got %q", lit4)
						stopformat = true
					}
					if len(lit4) != 1 {
						err = fmt.Errorf("Expecting a single character after MISSING=', got %q", lit4)
						stopformat = true
					}
					missing = []rune(lit4)[0]
				case GAP:
					tok3, lit3 := p.scanIgnoreWhitespace()
					if tok3 != EQUAL {
						err = fmt.Errorf("Expecting '=' after GAP, got %q", lit3)
						stopformat = true
					}
					tok4, lit4 := p.scanIgnoreWhitespace()
					if tok4 != IDENT {
						err = fmt.Errorf("Expecting an identifier after 'GAP=', got %q", lit4)
						stopformat = true
					}
					if len(lit4) != 1 {
						err = fmt.Errorf("Expecting a single character after GAP=', got %q", lit4)
						stopformat = true
					}
					gap = []rune(lit4)[0]
				default:
					if err = p.parseUnsupportedKey(lit2); err != nil {
						stopformat = true
					}
					treeio.LogWarning(fmt.Errorf("Unsupported key %q in %q command, skipping", lit2, lit))
				}
				if err != nil {
					stopdata = true
				}
			}
		case MATRIX:
			// Character matrix (Alignmemnt)
			// So far: Does not handle interleave case...
			stopmatrix := false
			for !stopmatrix {
				tok2, lit2 := p.scanIgnoreWhitespace()
				switch tok2 {
				case IDENT:
					//We remove whitespaces in sequences if any
					stopseq := false
					names = append(names, lit2)
					sequences = append(sequences, "")
					for !stopseq {
						tok3, lit3 := p.scanIgnoreWhitespace()
						switch tok3 {
						case IDENT:
							sequences[len(sequences)-1] = sequences[len(sequences)-1] + lit3
						case ENDOFLINE:
							stopseq = true
						default:
							err = fmt.Errorf("Expecting sequence after sequence identifier (%q) in Matrix block, got %q", lit2, lit3)
							stopseq = true
						}
					}
					if err != nil {
						stopmatrix = true
					}
				case ENDOFLINE:
					break
				case ENDOFCOMMAND:
					stopmatrix = true
				default:
					err = fmt.Errorf("Expecting sequence identifier in Matrix block, got %q", lit2)
					stopmatrix = true
				}
			}
			if err != nil {
				stopdata = true
			}

		default:
			err = p.parseUnsupportedCommand()
			treeio.LogWarning(fmt.Errorf("Unsupported command %q in block DATA, skipping", lit))
			if err != nil {
				stopdata = true
			}
		}
	}
	return
}

// Just skip the current command
func (p *Parser) parseUnsupportedCommand() (err error) {
	// Unsupported data command
	stopunsupported := false
	for !stopunsupported {
		tok, lit := p.scanIgnoreWhitespace()
		switch tok {
		case ILLEGAL:
			err = fmt.Errorf("found illegal token %q", lit)
			stopunsupported = true
		case EOF:
			err = fmt.Errorf("End of file within a command (no;)")
			stopunsupported = true
		case ENDOFCOMMAND:
			stopunsupported = true
		}
	}
	return
}

// Just skip the current key
func (p *Parser) parseUnsupportedKey(key string) (err error) {
	// Unsupported token
	tok, lit := p.scanIgnoreWhitespace()
	if tok != EQUAL {
		err = fmt.Errorf("Expecting '=' after %s, got %q", key, lit)
	} else {
		tok2, lit2 := p.scanIgnoreWhitespace()
		if tok2 != IDENT && tok2 != NUMERIC {
			err = fmt.Errorf("Expecting an identifier after '%s=', got %q", key, lit2)
		}
	}
	return
}

// Just skip the current block
func (p *Parser) parseUnsupportedBlock() error {
	var err error
	stopunsupported := false
	for !stopunsupported {
		tok, lit := p.scanIgnoreWhitespace()
		switch tok {
		case ILLEGAL:
			err = fmt.Errorf("found illegal token %q", lit)
			stopunsupported = true
		case EOF:
			err = fmt.Errorf("End of file within a block (no END;)")
			stopunsupported = true
		case END:
			tok2, _ := p.scanIgnoreWhitespace()
			if tok2 != ENDOFCOMMAND {
				err = fmt.Errorf("End token without ;")
			}
			stopunsupported = true
		}
	}
	return err
}