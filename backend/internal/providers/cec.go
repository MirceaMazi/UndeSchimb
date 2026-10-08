package providers

import (
	"fmt"
	"strings"
	"time"

	"github.com/undeschimb/undeschimb/internal/domain"
	"golang.org/x/net/html"
)

// ParseCECRates reads whole rows of the online-banking table. EUR also occurs
// in the introductory negotiation conditions, before any exchange-rate data.
// Column headings distinguish the bank's quotes from the BNR/BCE references.
func ParseCECRates(document string, fetchedAt time.Time) ([]domain.RateSnapshot, error) {
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return nil, fmt.Errorf("parse CEC online table: %w", err)
	}
	tables := cecNodes(root, func(node *html.Node) bool {
		return node.Data == "table" || cecHasClass(node, "custom-table")
	})
	for _, table := range tables {
		rows := cecNodes(table.FirstChild, func(node *html.Node) bool {
			return node.Data == "tr" || cecHasClass(node, "table-row")
		})
		currencyColumn, referenceColumn, buyColumn, sellColumn, columnCount := 0, -1, -1, -1, 0
		hasHeader := false
		for _, row := range rows {
			cells := cecCells(row)
			if len(cells) == 0 {
				continue
			}
			if !cecHasClass(row, "table-head") && cells[0].Data != "th" {
				continue
			}
			hasHeader = true
			columnCount = len(cells)
			for index, cell := range cells {
				switch strings.ToLower(cecText(cell)) {
				case "valuta", "moneda", "currency":
					currencyColumn = index
				case "curs bnr":
					referenceColumn = index
				case "cumpara", "cumpără", "cumparare", "cumpărare", "buy":
					buyColumn = index
				case "vinde", "vanzare", "vânzare", "sell":
					sellColumn = index
				}
			}
			break
		}
		if !hasHeader && table.Data == "table" {
			// Retain the legacy five-cell layout: code, name, BNR, buy, sell.
			referenceColumn, buyColumn, sellColumn, columnCount = 2, 3, 4, 5
		}
		if referenceColumn < 0 || buyColumn < 0 || sellColumn < 0 {
			continue
		}
		quotes := make(map[string]accountQuote, 4)
		for _, row := range rows {
			cells := cecCells(row)
			if len(cells) <= currencyColumn {
				continue
			}
			currency := ""
			for _, candidate := range []string{"EUR", "USD", "GBP", "CHF"} {
				if currencyPosition(cecText(cells[currencyColumn]), candidate) >= 0 {
					currency = candidate
					break
				}
			}
			if currency == "" {
				continue
			}
			if len(cells) != columnCount {
				return nil, fmt.Errorf("CEC %s has %d columns; expected %d", currency, len(cells), columnCount)
			}
			reference, referenceErr := parseRate(cecText(cells[referenceColumn]))
			buy, buyErr := parseRate(cecText(cells[buyColumn]))
			sell, sellErr := parseRate(cecText(cells[sellColumn]))
			if referenceErr != nil || !reference.IsPositive() || buyErr != nil || sellErr != nil || !validRetailQuote(buy, sell) {
				return nil, fmt.Errorf("invalid CEC %s reference/buy/sell cells", currency)
			}
			if previous, exists := quotes[currency]; exists && (!previous.Buy.Equal(buy) || !previous.Sell.Equal(sell)) {
				return nil, fmt.Errorf("CEC %s has conflicting rows", currency)
			}
			quotes[currency] = accountQuote{Buy: buy, Sell: sell}
		}
		if len(quotes) == 0 {
			continue
		}
		snapshots := make([]domain.RateSnapshot, 0, 4)
		for _, currency := range []string{"EUR", "USD", "GBP", "CHF"} {
			quote, exists := quotes[currency]
			if !exists {
				return nil, fmt.Errorf("CEC %s is missing from the online rate table", currency)
			}
			snapshots = append(snapshots, retailSnapshot("cec", currency, quote.Buy, quote.Sell, CECSourceURL, fetchedAt, fetchedAt))
		}
		return snapshots, nil
	}
	return nil, fmt.Errorf("CEC online rate table with reference/buy/sell columns is missing")
}

func cecNodes(node *html.Node, matches func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	for ; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode && matches(node) {
			result = append(result, node)
		} else {
			result = append(result, cecNodes(node.FirstChild, matches)...)
		}
	}
	return result
}

func cecHasClass(node *html.Node, class string) bool {
	for _, attribute := range node.Attr {
		if attribute.Key == "class" {
			for _, value := range strings.Fields(attribute.Val) {
				if value == class {
					return true
				}
			}
		}
	}
	return false
}

func cecCells(row *html.Node) []*html.Node {
	var cells []*html.Node
	for node := row.FirstChild; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode && (node.Data == "td" || node.Data == "th" || cecHasClass(node, "col")) {
			cells = append(cells, node)
		}
	}
	return cells
}

func cecText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	if node.Data == "script" || node.Data == "style" {
		return ""
	}
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(cecText(child))
		text.WriteByte(' ')
	}
	return strings.Join(strings.Fields(text.String()), " ")
}
