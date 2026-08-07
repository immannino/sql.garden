package main

import (
	"embed"
	"fmt"
	"os"
)

//go:embed sampledata
var sampleDataFS embed.FS

// SampleDataset describes a bundled demo dataset.
type SampleDataset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Tables      []string `json:"tables"`
	RowCount    int      `json:"rowCount"`
}

var sampleDatasets = []SampleDataset{
	{
		ID:          "f1",
		Name:        "F1 2023 Season",
		Description: "Driver standings, race results, and team performance across all 22 rounds of the 2023 Formula 1 World Championship.",
		Icon:        "🏎️",
		Tables:      []string{"f1_drivers", "f1_teams", "f1_races", "f1_results"},
		RowCount:    492,
	},
	{
		ID:          "markets",
		Name:        "Stock Markets",
		Description: "Daily OHLCV price data for 10 tickers — AAPL, MSFT, GOOGL, AMZN, TSLA, NVDA and more — from January 2020 through December 2024.",
		Icon:        "📈",
		Tables:      []string{"mkt_companies", "mkt_prices"},
		RowCount:    13050,
	},
	{
		ID:          "economy",
		Name:        "Macroeconomics",
		Description: "Monthly US economic indicators from 2014–2024: GDP growth, unemployment, CPI inflation, Fed funds rate, and S&P 500.",
		Icon:        "🏛️",
		Tables:      []string{"eco_indicators"},
		RowCount:    132,
	},
	{
		ID:          "ecommerce",
		Name:        "E-commerce",
		Description: "A small online store: 10 users, 10 products, 66 orders, and 175 line items across 12 months. Great for exploring JOIN patterns.",
		Icon:        "🛒",
		Tables:      []string{"ec_users", "ec_products", "ec_orders", "ec_items"},
		RowCount:    261,
	},
	{
		ID:          "weather",
		Name:        "Global Weather",
		Description: "Daily high/low temperature, precipitation, and conditions for New York, London, Tokyo, Sydney, and Cairo from 2021–2023.",
		Icon:        "🌤️",
		Tables:      []string{"wx_daily"},
		RowCount:    5475,
	},
}

func (a *App) ListSampleDatasets() []SampleDataset {
	return sampleDatasets
}

// loadEmbeddedCSV writes an embedded CSV to a temp file and CREATE TABLE AS SELECTs it into DuckDB.
func (a *App) loadEmbeddedCSV(fsPath, tableName string) error {
	data, err := sampleDataFS.ReadFile(fsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", fsPath, err)
	}
	tmp, err := os.CreateTemp("", "sqg_*.csv")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	_, _ = a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName))
	_, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`CREATE TABLE "%s" AS SELECT * FROM read_csv_auto('%s', header=true)`,
		tableName, escapeSingleQuote(tmp.Name()),
	))
	return err
}

// at returns a CanvasAction helper with explicit canvas position.
func at(action CanvasAction, x, y float64) CanvasAction {
	action.HasPosition = true
	action.X = x
	action.Y = y
	return action
}

// tableAction builds a positioned table CanvasAction with columns and row count
// populated by querying the already-loaded DuckDB table.
func (a *App) tableAction(nodeID, tableName string, x, y float64) CanvasAction {
	cols, _ := a.GetTableInfo(tableName)
	var canvasCols []CanvasColumn
	for _, c := range cols {
		canvasCols = append(canvasCols, CanvasColumn{Name: c.Name, Type: c.Type})
	}

	var count int64
	row := a.duck.QueryRowContext(a.ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName))
	_ = row.Scan(&count)

	return CanvasAction{
		Type:        "table",
		NodeID:      nodeID,
		Name:        tableName,
		Columns:     canvasCols,
		RowCount:    count,
		HasPosition: true,
		X:           x,
		Y:           y,
	}
}

// LoadSampleDataset imports tables and returns pre-built canvas actions for the chosen dataset.
func (a *App) LoadSampleDataset(id string) ([]CanvasAction, error) {
	switch id {
	case "f1":
		return a.loadF1Dataset()
	case "markets":
		return a.loadMarketsDataset()
	case "economy":
		return a.loadEconomyDataset()
	case "ecommerce":
		return a.loadEcommerceDataset()
	case "weather":
		return a.loadWeatherDataset()
	default:
		return nil, fmt.Errorf("unknown dataset: %s", id)
	}
}

// ── F1 ───────────────────────────────────────────────────────────────────────

func (a *App) loadF1Dataset() ([]CanvasAction, error) {
	tables := []struct{ file, name string }{
		{"sampledata/f1/drivers.csv", "f1_drivers"},
		{"sampledata/f1/teams.csv", "f1_teams"},
		{"sampledata/f1/races.csv", "f1_races"},
		{"sampledata/f1/results.csv", "f1_results"},
	}
	for _, t := range tables {
		if err := a.loadEmbeddedCSV(t.file, t.name); err != nil {
			return nil, err
		}
		a.SaveTableData(t.name) //nolint:errcheck
	}

	const (
		standingsID   = "demo_f1_standings"
		progressionID = "demo_f1_progression"
	)

	standingsSQL := `SELECT
    d.name        AS driver,
    d.team        AS team,
    SUM(r.points) AS total_points,
    COUNT(CASE WHEN r.position = 1 THEN 1 END)        AS wins,
    COUNT(CASE WHEN TRY_CAST(r.position AS INT) <= 3 THEN 1 END) AS podiums
FROM f1_results r
JOIN f1_drivers d ON r.driver_id = d.driver_id
WHERE r.status = 'Finished' OR r.points > 0
GROUP BY d.driver_id, d.name, d.team
ORDER BY total_points DESC`

	progressionSQL := `WITH top5 AS (
    SELECT d.driver_id
    FROM f1_results r
    JOIN f1_drivers d ON r.driver_id = d.driver_id
    GROUP BY d.driver_id
    ORDER BY SUM(r.points) DESC
    LIMIT 5
)
SELECT
    ra.round,
    d.name AS driver,
    SUM(r.points) OVER (PARTITION BY d.driver_id ORDER BY ra.round) AS cumulative_points
FROM f1_results r
JOIN f1_drivers d  ON r.driver_id = d.driver_id
JOIN f1_races   ra ON r.race_id   = ra.race_id
WHERE d.driver_id IN (SELECT driver_id FROM top5)
ORDER BY ra.round, cumulative_points DESC`

	constructorSQL := `SELECT
    d.team AS constructor,
    SUM(r.points) AS points
FROM f1_results r
JOIN f1_drivers d ON r.driver_id = d.driver_id
GROUP BY d.team
ORDER BY points DESC`

	winsSQL := `SELECT
    d.name AS driver,
    COUNT(*) AS wins
FROM f1_results r
JOIN f1_drivers d ON r.driver_id = d.driver_id
WHERE r.position = 1
GROUP BY d.driver_id, d.name
ORDER BY wins DESC`

	// Layout constants
	const (
		colA = 0.0    // query nodes
		colB = 360.0  // sourced chart nodes
		colC = 760.0  // inline chart nodes / markdown
		tblY = 160.0  // schema strip
		sec1 = 420.0  // first query/chart row
		sec2 = 790.0  // second query/chart row
		mdH  = 100.0  // markdown node height
	)

	return []CanvasAction{
		// ── Intro ────────────────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_f1_intro", Name: "f1_intro",
			Content: "# 🏎️ F1 2023 Season\n\nFormula 1 race results across all 22 rounds of the 2023 World Championship. Explore driver standings, team performance, and points progression using **QueryNodes** that feed data into **Charts** — or write SQL directly inside a chart.",
		}, colA, 0),

		// ── Schema strip ─────────────────────────────────────────────────────
		a.tableAction("demo_f1_tbl_drivers", "f1_drivers", colA, tblY),
		a.tableAction("demo_f1_tbl_teams", "f1_teams", 250, tblY),
		a.tableAction("demo_f1_tbl_races", "f1_races", 500, tblY),
		a.tableAction("demo_f1_tbl_results", "f1_results", 740, tblY),

		// ── Pipeline label ───────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_f1_md1", Name: "f1_pipeline_label",
			Content: "## Query → Chart\nA **QueryNode** runs SQL and caches results. A **ChartNode** can source from it — no duplicate queries.",
		}, colA, sec1-mdH-10),
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_f1_md2", Name: "f1_inline_label",
			Content: "## Inline SQL → Chart\nA ChartNode can also run its own SQL directly — handy for simple one-off visualisations.",
		}, colC, sec1-mdH-10),

		// ── Standings: query + sourced chart ─────────────────────────────────
		at(CanvasAction{
			Type: "query", NodeID: standingsID, Name: "Driver Standings",
			SQL: standingsSQL,
		}, colA, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_f1_standings_chart", Name: "Championship Points",
			SourceID: standingsID, ChartType: "barY",
			XColumn: "driver", YColumn: "total_points",
		}, colB, sec1),

		// ── Inline: constructor + wins ────────────────────────────────────────
		at(CanvasAction{
			Type: "chart", NodeID: "demo_f1_constructors", Name: "Constructor Standings",
			SQL: constructorSQL, ChartType: "barY",
			XColumn: "constructor", YColumn: "points",
		}, colC, sec1),

		// ── Progression: query + sourced chart ───────────────────────────────
		at(CanvasAction{
			Type: "query", NodeID: progressionID, Name: "Points Progression (Top 5)",
			SQL: progressionSQL,
		}, colA, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_f1_progression_chart", Name: "Season Points Race",
			SourceID: progressionID, ChartType: "lineY",
			XColumn: "round", YColumn: "cumulative_points", ColorColumn: "driver",
		}, colB, sec2),

		// ── Inline: race wins ─────────────────────────────────────────────────
		at(CanvasAction{
			Type: "chart", NodeID: "demo_f1_wins", Name: "Race Wins",
			SQL: winsSQL, ChartType: "barY",
			XColumn: "driver", YColumn: "wins",
		}, colC, sec2),
	}, nil
}

// ── Markets ───────────────────────────────────────────────────────────────────

func (a *App) loadMarketsDataset() ([]CanvasAction, error) {
	tables := []struct{ file, name string }{
		{"sampledata/markets/companies.csv", "mkt_companies"},
		{"sampledata/markets/prices.csv", "mkt_prices"},
	}
	for _, t := range tables {
		if err := a.loadEmbeddedCSV(t.file, t.name); err != nil {
			return nil, err
		}
		a.SaveTableData(t.name) //nolint:errcheck
	}

	const histID = "demo_mkt_history"

	histSQL := `SELECT date, ticker, close
FROM mkt_prices
ORDER BY date, ticker`

	returnsSQL := `WITH first_last AS (
    SELECT ticker,
        FIRST(close ORDER BY date) AS price_start,
        LAST(close  ORDER BY date) AS price_end
    FROM mkt_prices
    GROUP BY ticker
)
SELECT
    c.name AS company,
    c.sector,
    f.ticker,
    ROUND((f.price_end - f.price_start) / f.price_start * 100, 1) AS total_return_pct
FROM first_last f
JOIN mkt_companies c ON f.ticker = c.ticker
ORDER BY total_return_pct DESC`

	volumeSQL := `SELECT
    ticker,
    ROUND(AVG(volume) / 1e6, 2) AS avg_daily_vol_m
FROM mkt_prices
GROUP BY ticker
ORDER BY avg_daily_vol_m DESC`

	volatilitySQL := `WITH daily_returns AS (
    SELECT
        ticker,
        close / LAG(close) OVER (PARTITION BY ticker ORDER BY date) - 1 AS daily_return
    FROM mkt_prices
)
SELECT
    ticker,
    ROUND(STDDEV(daily_return) * SQRT(252) * 100, 1) AS annual_vol_pct
FROM daily_returns
WHERE daily_return IS NOT NULL
GROUP BY ticker
ORDER BY annual_vol_pct DESC
LIMIT 10`

	const (
		colA = 0.0
		colB = 360.0
		colC = 760.0
		tblY = 160.0
		sec1 = 420.0
		sec2 = 790.0
		mdH  = 100.0
	)

	return []CanvasAction{
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_mkt_intro", Name: "mkt_intro",
			Content: "# 📈 Stock Markets\n\nDaily OHLCV data for 10 tickers from January 2020 through December 2024. Explore price history, total returns, volatility, and trading volume with DuckDB's window functions.",
		}, colA, 0),

		a.tableAction("demo_mkt_tbl_co", "mkt_companies", colA, tblY),
		a.tableAction("demo_mkt_tbl_px", "mkt_prices", 260, tblY),

		at(CanvasAction{
			Type: "markdown", NodeID: "demo_mkt_md1", Name: "mkt_pipeline_label",
			Content: "## Query → Chart\nPrice history sourced from a QueryNode — change the SQL once, all charts update.",
		}, colA, sec1-mdH-10),
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_mkt_md2", Name: "mkt_inline_label",
			Content: "## Inline SQL → Chart\nQuick one-off charts with SQL written directly in the node.",
		}, colC, sec1-mdH-10),

		at(CanvasAction{
			Type: "query", NodeID: histID, Name: "Price History",
			SQL: histSQL,
		}, colA, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_mkt_hist_chart", Name: "Close Price by Ticker",
			SourceID: histID, ChartType: "lineY",
			XColumn: "date", YColumn: "close", ColorColumn: "ticker",
		}, colB, sec1),

		at(CanvasAction{
			Type: "chart", NodeID: "demo_mkt_returns", Name: "Total Return %",
			SQL: returnsSQL, ChartType: "barY",
			XColumn: "ticker", YColumn: "total_return_pct", ColorColumn: "sector",
		}, colC, sec1),

		at(CanvasAction{
			Type: "query", NodeID: "demo_mkt_vol_query", Name: "Volatility",
			SQL: volatilitySQL,
		}, colA, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_mkt_vol_chart", Name: "Annualised Volatility %",
			SourceID: "demo_mkt_vol_query", ChartType: "barY",
			XColumn: "ticker", YColumn: "annual_vol_pct",
		}, colB, sec2),

		at(CanvasAction{
			Type: "chart", NodeID: "demo_mkt_avgvol", Name: "Avg Daily Volume (M)",
			SQL: volumeSQL, ChartType: "barY",
			XColumn: "ticker", YColumn: "avg_daily_vol_m",
		}, colC, sec2),
	}, nil
}

// ── Economy ───────────────────────────────────────────────────────────────────

func (a *App) loadEconomyDataset() ([]CanvasAction, error) {
	if err := a.loadEmbeddedCSV("sampledata/economy/indicators.csv", "eco_indicators"); err != nil {
		return nil, err
	}
	a.SaveTableData("eco_indicators") //nolint:errcheck

	const ecoID = "demo_eco_all"

	allSQL := `SELECT date, gdp_growth_pct, unemployment_pct, cpi_yoy_pct, fed_funds_rate, sp500_close
FROM eco_indicators ORDER BY date`

	cpiSQL := `SELECT date, cpi_yoy_pct FROM eco_indicators ORDER BY date`
	ffrSQL := `SELECT date, fed_funds_rate FROM eco_indicators ORDER BY date`

	gdpSQL := `SELECT
    STRFTIME(date, '%Y') AS year,
    ROUND(AVG(gdp_growth_pct), 2) AS avg_gdp_growth
FROM eco_indicators
GROUP BY year ORDER BY year`

	const (
		colA = 0.0
		colB = 360.0
		colC = 760.0
		tblY = 160.0
		sec1 = 420.0
		sec2 = 790.0
		mdH  = 100.0
	)

	return []CanvasAction{
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_eco_intro", Name: "eco_intro",
			Content: "# 🏛️ US Macroeconomics\n\nMonthly indicators from 2014–2024. Watch how the Fed funds rate responded to the 2020 COVID shock and 2022 inflation surge. GDP, unemployment, CPI, and S&P 500 all in one table.",
		}, colA, 0),

		a.tableAction("demo_eco_tbl", "eco_indicators", colA, tblY),

		at(CanvasAction{
			Type: "markdown", NodeID: "demo_eco_md1", Name: "eco_pipeline_label",
			Content: "## Query → Chart\nOne QueryNode fetches everything; individual charts slice different columns from the same result.",
		}, colA, sec1-mdH-10),
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_eco_md2", Name: "eco_inline_label",
			Content: "## Inline SQL → Chart\nFed funds rate and GDP aggregated with inline SQL — no shared QueryNode needed.",
		}, colC, sec1-mdH-10),

		at(CanvasAction{
			Type: "query", NodeID: ecoID, Name: "All Indicators",
			SQL: allSQL,
		}, colA, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_eco_unemp", Name: "Unemployment Rate %",
			SourceID: ecoID, ChartType: "lineY",
			XColumn: "date", YColumn: "unemployment_pct",
		}, colB, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_eco_cpi_inline", Name: "CPI Inflation YoY %",
			SQL: cpiSQL, ChartType: "lineY",
			XColumn: "date", YColumn: "cpi_yoy_pct",
		}, colC, sec1),

		at(CanvasAction{
			Type: "query", NodeID: "demo_eco_ffr_query", Name: "Fed Funds Rate",
			SQL: ffrSQL,
		}, colA, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_eco_ffr_chart", Name: "Fed Funds Rate %",
			SourceID: "demo_eco_ffr_query", ChartType: "areaY",
			XColumn: "date", YColumn: "fed_funds_rate",
		}, colB, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_eco_gdp", Name: "Avg GDP Growth by Year",
			SQL: gdpSQL, ChartType: "barY",
			XColumn: "year", YColumn: "avg_gdp_growth",
		}, colC, sec2),
	}, nil
}

// ── E-commerce ────────────────────────────────────────────────────────────────

func (a *App) loadEcommerceDataset() ([]CanvasAction, error) {
	tables := []struct{ file, name string }{
		{"sampledata/ecommerce/users.csv", "ec_users"},
		{"sampledata/ecommerce/products.csv", "ec_products"},
		{"sampledata/ecommerce/orders.csv", "ec_orders"},
		{"sampledata/ecommerce/items.csv", "ec_items"},
	}
	for _, t := range tables {
		if err := a.loadEmbeddedCSV(t.file, t.name); err != nil {
			return nil, err
		}
		a.SaveTableData(t.name) //nolint:errcheck
	}

	const revenueID = "demo_ec_revenue"

	revenueSQL := `SELECT
    p.name    AS product,
    p.category,
    SUM(i.quantity * i.unit_price) AS revenue,
    SUM(i.quantity)                AS units_sold
FROM ec_items i
JOIN ec_products p ON i.product_id = p.product_id
GROUP BY p.product_id, p.name, p.category
ORDER BY revenue DESC`

	monthlySQL := `SELECT
    STRFTIME(o.created_at, '%Y-%m') AS month,
    COUNT(DISTINCT o.order_id)      AS orders,
    ROUND(SUM(o.total), 2)          AS revenue
FROM ec_orders o
WHERE o.status != 'cancelled'
GROUP BY month ORDER BY month`

	ltvSQL := `SELECT
    u.name,
    u.plan,
    COUNT(DISTINCT o.order_id)    AS orders,
    ROUND(SUM(o.total), 2)        AS lifetime_value
FROM ec_orders o
JOIN ec_users u ON o.user_id = u.user_id
WHERE o.status != 'cancelled'
GROUP BY u.user_id, u.name, u.plan
ORDER BY lifetime_value DESC`

	categorySQL := `SELECT
    p.category,
    ROUND(SUM(i.quantity * i.unit_price), 2) AS revenue
FROM ec_items i
JOIN ec_products p ON i.product_id = p.product_id
GROUP BY p.category
ORDER BY revenue DESC`

	const (
		colA = 0.0
		colB = 360.0
		colC = 760.0
		tblY = 160.0
		sec1 = 420.0
		sec2 = 790.0
		mdH  = 100.0
	)

	return []CanvasAction{
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_ec_intro", Name: "ec_intro",
			Content: "# 🛒 E-commerce\n\nA small online store with users, products, orders, and line items across 12 months. Great for practising multi-table JOINs, GROUP BY aggregations, and revenue analysis.",
		}, colA, 0),

		a.tableAction("demo_ec_tbl_u", "ec_users", colA, tblY),
		a.tableAction("demo_ec_tbl_p", "ec_products", 240, tblY),
		a.tableAction("demo_ec_tbl_o", "ec_orders", 480, tblY),
		a.tableAction("demo_ec_tbl_i", "ec_items", 720, tblY),

		at(CanvasAction{
			Type: "markdown", NodeID: "demo_ec_md1", Name: "ec_pipeline_label",
			Content: "## Query → Chart\nRevenue by product sourced from a shared QueryNode that also powers the units-sold view.",
		}, colA, sec1-mdH-10),
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_ec_md2", Name: "ec_inline_label",
			Content: "## Inline SQL → Chart\nCategory breakdown and monthly trend written directly in each chart.",
		}, colC, sec1-mdH-10),

		at(CanvasAction{
			Type: "query", NodeID: revenueID, Name: "Revenue by Product",
			SQL: revenueSQL,
		}, colA, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_ec_rev_chart", Name: "Product Revenue",
			SourceID: revenueID, ChartType: "barY",
			XColumn: "product", YColumn: "revenue", ColorColumn: "category",
		}, colB, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_ec_cat", Name: "Revenue by Category",
			SQL: categorySQL, ChartType: "barY",
			XColumn: "category", YColumn: "revenue",
		}, colC, sec1),

		at(CanvasAction{
			Type: "query", NodeID: "demo_ec_monthly_q", Name: "Monthly Revenue",
			SQL: monthlySQL,
		}, colA, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_ec_monthly_chart", Name: "Monthly Revenue Trend",
			SourceID: "demo_ec_monthly_q", ChartType: "lineY",
			XColumn: "month", YColumn: "revenue",
		}, colB, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_ec_ltv", Name: "Customer Lifetime Value",
			SQL: ltvSQL, ChartType: "barY",
			XColumn: "name", YColumn: "lifetime_value", ColorColumn: "plan",
		}, colC, sec2),
	}, nil
}

// ── Learn Tracks ─────────────────────────────────────────────────────────────

// LearnTrack describes an interactive SQL learning canvas.
type LearnTrack struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Level       string   `json:"level"`
	Chapters    int      `json:"chapters"`
	Tables      []string `json:"tables"`
	RowCount    int      `json:"rowCount"`
	Tags        []string `json:"tags"`
}

var learnTracks = []LearnTrack{
	{
		ID:          "ecommerce-sql",
		Name:        "SQL Fundamentals: E-Commerce",
		Description: "Learn SQL from SELECT to CTEs using a realistic store dataset: 100 customers, 40 products, ~500 orders, and ~1,100 order items across 8 interactive chapters.",
		Icon:        "🛍️",
		Level:       "Beginner",
		Chapters:    8,
		Tables:      []string{"lrn_customers", "lrn_products", "lrn_orders", "lrn_order_items"},
		RowCount:    1740,
		Tags:        []string{"SELECT", "WHERE", "GROUP BY", "JOIN", "Window Functions", "CTE"},
	},
	{
		ID:          "sqlbolt-lesson1",
		Name:        "SQLBolt Lesson 1: SELECT 101",
		Description: "5 interactive SELECT exercises adapted from SQLBolt (sqlbolt.com) by Nick Ciubotariu, included here for educational demonstration. For the full free course — WHERE, JOINs, aggregates and more — visit sqlbolt.com.",
		Icon:        "🎬",
		Level:       "Beginner",
		Chapters:    1,
		Tables:      []string{"sb_movies"},
		RowCount:    14,
		Tags:        []string{"SELECT", "columns", "wildcard"},
	},
}

func (a *App) ListLearnTracks() []LearnTrack {
	return learnTracks
}

func (a *App) LoadLearnTrack(id string) ([]CanvasAction, error) {
	switch id {
	case "ecommerce-sql":
		return a.loadLearnEcommerceTrack()
	case "sqlbolt-lesson1":
		return a.loadLearnSQLBoltLesson1()
	default:
		return nil, fmt.Errorf("unknown learn track: %s", id)
	}
}

func (a *App) loadLearnEcommerceTrack() ([]CanvasAction, error) {
	stmts := []string{
		`DROP TABLE IF EXISTS lrn_order_items`,
		`DROP TABLE IF EXISTS lrn_orders`,
		`DROP TABLE IF EXISTS lrn_customers`,
		`DROP TABLE IF EXISTS lrn_products`,

		// 40 products across 5 categories
		`CREATE TABLE lrn_products AS SELECT * FROM (VALUES
  (1,'Laptop Pro 15"','Electronics',1299.99),
  (2,'Wireless Mouse','Electronics',29.99),
  (3,'Mechanical Keyboard','Electronics',89.99),
  (4,'USB-C Hub','Electronics',49.99),
  (5,'4K Monitor 27"','Electronics',399.99),
  (6,'Noise-Cancelling Headphones','Electronics',149.99),
  (7,'Webcam 1080p','Electronics',69.99),
  (8,'SSD 1TB','Electronics',99.99),
  (9,'Smart Speaker','Electronics',59.99),
  (10,'Phone Stand','Electronics',19.99),
  (11,'Running Shoes','Sports',89.99),
  (12,'Yoga Mat','Sports',34.99),
  (13,'Water Bottle 32oz','Sports',24.99),
  (14,'Resistance Bands','Sports',18.99),
  (15,'Jump Rope','Sports',9.99),
  (16,'Protein Powder 5lb','Sports',54.99),
  (17,'Gym Gloves','Sports',14.99),
  (18,'Foam Roller','Sports',29.99),
  (19,'Pull-Up Bar','Sports',39.99),
  (20,'Fitness Tracker','Sports',79.99),
  (21,'SQL for Beginners','Books',19.99),
  (22,'The Data Warehouse Toolkit','Books',49.99),
  (23,'Python Crash Course','Books',34.99),
  (24,'Designing Data-Intensive Apps','Books',59.99),
  (25,'Clean Code','Books',39.99),
  (26,'The Pragmatic Programmer','Books',44.99),
  (27,'Learning SQL','Books',37.99),
  (28,'Database Internals','Books',49.99),
  (29,'Desk Organizer','Home',29.99),
  (30,'LED Desk Lamp','Home',39.99),
  (31,'Coffee Mug 16oz','Home',12.99),
  (32,'Succulent Plant Set','Home',24.99),
  (33,'Whiteboard 36x24','Home',54.99),
  (34,'Cable Management Box','Home',21.99),
  (35,'Standing Desk Mat','Home',44.99),
  (36,'T-Shirt Pack of 5','Clothing',34.99),
  (37,'Hoodie Classic','Clothing',49.99),
  (38,'Chino Pants','Clothing',59.99),
  (39,'Compression Socks 3pk','Clothing',14.99),
  (40,'Baseball Cap','Clothing',22.99)
) t(id,name,category,price)`,

		// 100 customers, deterministic names/states
		`CREATE TABLE lrn_customers AS
SELECT
  i AS id,
  CASE i%20 WHEN 0 THEN 'Alice' WHEN 1 THEN 'Bob' WHEN 2 THEN 'Carol'
    WHEN 3 THEN 'David' WHEN 4 THEN 'Emma' WHEN 5 THEN 'Frank'
    WHEN 6 THEN 'Grace' WHEN 7 THEN 'Henry' WHEN 8 THEN 'Iris'
    WHEN 9 THEN 'Jack' WHEN 10 THEN 'Kate' WHEN 11 THEN 'Liam'
    WHEN 12 THEN 'Mia' WHEN 13 THEN 'Noah' WHEN 14 THEN 'Olivia'
    WHEN 15 THEN 'Paul' WHEN 16 THEN 'Quinn' WHEN 17 THEN 'Rachel'
    WHEN 18 THEN 'Sam' ELSE 'Tara' END || ' ' ||
  CASE i%15 WHEN 0 THEN 'Smith' WHEN 1 THEN 'Johnson' WHEN 2 THEN 'Williams'
    WHEN 3 THEN 'Brown' WHEN 4 THEN 'Jones' WHEN 5 THEN 'Garcia'
    WHEN 6 THEN 'Miller' WHEN 7 THEN 'Davis' WHEN 8 THEN 'Wilson'
    WHEN 9 THEN 'Taylor' WHEN 10 THEN 'Anderson' WHEN 11 THEN 'Thomas'
    WHEN 12 THEN 'Jackson' WHEN 13 THEN 'White' ELSE 'Harris' END AS name,
  'user' || i || '@example.com' AS email,
  CASE i%10 WHEN 0 THEN 'NY' WHEN 1 THEN 'CA' WHEN 2 THEN 'TX' WHEN 3 THEN 'FL'
    WHEN 4 THEN 'IL' WHEN 5 THEN 'PA' WHEN 6 THEN 'OH' WHEN 7 THEN 'GA'
    WHEN 8 THEN 'NC' ELSE 'MI' END AS state,
  DATE '2023-01-01' + ((i*37)%365) * INTERVAL '1 day' AS joined_at
FROM generate_series(1,100) t(i)`,

		// ~499 orders, seasonal pattern (Nov/Dec 3× heavier), power customers 1-15
		`CREATE TABLE lrn_orders AS
WITH months(m,cnt) AS (VALUES
  (1,30),(2,28),(3,33),(4,31),(5,35),(6,36),
  (7,40),(8,38),(9,33),(10,45),(11,70),(12,80)
),
slots AS (
  SELECT row_number() OVER () AS slot, m AS month_num
  FROM months, generate_series(1,cnt) gs(j)
)
SELECT
  slot AS id,
  CASE WHEN slot%3=0 THEN (slot%15)+1 ELSE ((slot*7+11)%85)+16 END AS customer_id,
  DATE '2023-01-01' + (month_num-1) * INTERVAL '1 month'
    + ((slot%27)+1) * INTERVAL '1 day' AS created_at,
  CASE WHEN slot%11=0 THEN 'cancelled' ELSE 'completed' END AS status,
  0.0::DOUBLE AS total
FROM slots`,

		// ~1,100 order items: 2/order, 3 for id%3=0, 1 for cancelled
		`CREATE TABLE lrn_order_items AS
WITH items AS (
  SELECT o.id AS order_id, gs.j AS item_num
  FROM lrn_orders o CROSS JOIN generate_series(1,2) gs(j)
  WHERE o.status='completed'
  UNION ALL
  SELECT o.id, 1 FROM lrn_orders o WHERE o.status='cancelled'
  UNION ALL
  SELECT o.id, 3 FROM lrn_orders o WHERE o.status='completed' AND o.id%3=0
)
SELECT
  row_number() OVER (ORDER BY order_id,item_num) AS id,
  order_id,
  ((order_id*7+item_num*13-1)%40)+1 AS product_id,
  (item_num%3)+1 AS quantity,
  p.price AS unit_price
FROM items
JOIN lrn_products p ON p.id=((order_id*7+item_num*13-1)%40)+1
ORDER BY order_id,item_num`,

		// Populate order totals from items
		`UPDATE lrn_orders SET total=(
  SELECT COALESCE(ROUND(SUM(quantity*unit_price),2),0)
  FROM lrn_order_items WHERE order_id=lrn_orders.id
)`,
	}

	for _, s := range stmts {
		if _, err := a.duck.ExecContext(a.ctx, s); err != nil {
			return nil, fmt.Errorf("learn track setup: %w", err)
		}
	}
	for _, tbl := range []string{"lrn_products", "lrn_customers", "lrn_orders", "lrn_order_items"} {
		a.SaveTableData(tbl) //nolint:errcheck
	}

	// ── Chapter positions ──────────────────────────────────────────────────────
	// Chapters read left-to-right. ch0 is wider (schema overview), ch1+ are 800px apart.
	const (
		ch0 = 0.0
		ch1 = 1000.0
		ch2 = 1800.0
		ch3 = 2600.0
		ch4 = 3400.0
		ch5 = 4200.0
		ch6 = 5000.0
		ch7 = 5800.0
		qY  = 240.0 // query/chart row Y
		chY = 300.0 // chart X offset within chapter (right of query)
	)

	return []CanvasAction{
		// ── Ch 0: Introduction & Schema ───────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_intro_md", Name: "intro",
			Content: "# 🛍️ SQL Fundamentals: E-Commerce\n\nLearn SQL by exploring a realistic store: **100 customers**, **40 products**, **~500 orders**, and **~1,100 order items** generated fresh in your local DuckDB.\n\nScroll right through **8 chapters** — each introduces a new SQL concept. Run the query, read the results, then try the exercises in the comments.\n\n**Tables:** `lrn_customers` · `lrn_products` · `lrn_orders` · `lrn_order_items`",
		}, ch0, 0),

		// Schema strip
		a.tableAction("lrn_tbl_customers", "lrn_customers", ch0, qY),
		a.tableAction("lrn_tbl_products", "lrn_products", ch0+205, qY),
		a.tableAction("lrn_tbl_orders", "lrn_orders", ch0+415, qY),
		a.tableAction("lrn_tbl_items", "lrn_order_items", ch0+630, qY),

		// Dataset summary
		at(CanvasAction{
			Type: "query", NodeID: "lrn_setup_q", Name: "Dataset Summary",
			SQL: `SELECT 'lrn_customers'  AS "table", COUNT(*) AS rows FROM lrn_customers
UNION ALL SELECT 'lrn_products',  COUNT(*) FROM lrn_products
UNION ALL SELECT 'lrn_orders',    COUNT(*) FROM lrn_orders
UNION ALL SELECT 'lrn_order_items', COUNT(*) FROM lrn_order_items`,
		}, ch0, qY+200),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_setup_chart", Name: "Row counts",
			SourceID: "lrn_setup_q", ChartType: "barY",
			XColumn: "table", YColumn: "rows",
		}, ch0+chY, qY+200),

		// ── Ch 1: SELECT ──────────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch1_md", Name: "Ch 1: SELECT",
			Content: "## Ch 1 — SELECT\n\n`SELECT` retrieves columns from a table. `ORDER BY` sorts rows; `*` selects all columns.\n\n**Try:**\n- Sort by `price ASC` instead\n- Replace the column list with `*`\n- Add `LIMIT 5` at the end",
		}, ch1, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch1_q", Name: "Ch 1: Products by Category",
			SQL: `-- Select product name, category, and price, sorted by category then price
SELECT name, category, price
FROM lrn_products
ORDER BY category, price DESC`,
		}, ch1, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch1_chart", Name: "Product Prices",
			SourceID: "lrn_ch1_q", ChartType: "barY",
			XColumn: "name", YColumn: "price", ColorColumn: "category",
		}, ch1+chY, qY),

		// ── Ch 2: WHERE ───────────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch2_md", Name: "Ch 2: WHERE",
			Content: "## Ch 2 — WHERE\n\n`WHERE` filters rows before returning them. Combine conditions with `AND` / `OR`.\n\n**Try:**\n- Change `'Electronics'` to `'Books'`\n- Replace `< 100` with `BETWEEN 50 AND 200`\n- Remove the category filter entirely",
		}, ch2, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch2_q", Name: "Ch 2: Budget Electronics",
			SQL: `-- Find Electronics products under $100
SELECT name, price
FROM lrn_products
WHERE category = 'Electronics'
  AND price < 100
ORDER BY price ASC`,
		}, ch2, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch2_chart", Name: "Budget Electronics",
			SourceID: "lrn_ch2_q", ChartType: "barY",
			XColumn: "name", YColumn: "price",
		}, ch2+chY, qY),

		// ── Ch 3: ORDER BY + LIMIT ────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch3_md", Name: "Ch 3: ORDER BY & LIMIT",
			Content: "## Ch 3 — ORDER BY & LIMIT\n\n`ORDER BY col DESC` sorts highest-first. `LIMIT N` caps the row count — useful for top-N reports.\n\n**Try:**\n- Change `DESC` to `ASC` (cheapest first)\n- Try `LIMIT 5` or `LIMIT 20`\n- Sort by `name` alphabetically",
		}, ch3, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch3_q", Name: "Ch 3: Top 10 by Price",
			SQL: `-- Top 10 most expensive products
SELECT name, category, price
FROM lrn_products
ORDER BY price DESC
LIMIT 10`,
		}, ch3, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch3_chart", Name: "Top 10 Priciest Products",
			SourceID: "lrn_ch3_q", ChartType: "barY",
			XColumn: "name", YColumn: "price", ColorColumn: "category",
		}, ch3+chY, qY),

		// ── Ch 4: GROUP BY ────────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch4_md", Name: "Ch 4: GROUP BY",
			Content: "## Ch 4 — GROUP BY\n\n`GROUP BY` collapses rows into groups. Aggregates (`COUNT`, `SUM`, `AVG`, `MIN`, `MAX`) summarise each group.\n\n**Try:**\n- Group by `status` instead of month\n- Replace `SUM` with `AVG` for average order value\n- Add `HAVING revenue > 2000` after `GROUP BY`",
		}, ch4, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch4_q", Name: "Ch 4: Monthly Revenue",
			SQL: `-- Orders and revenue by month
SELECT
  STRFTIME(created_at, '%Y-%m') AS month,
  COUNT(*)                       AS total_orders,
  ROUND(SUM(total), 2)           AS revenue
FROM lrn_orders
WHERE status = 'completed'
GROUP BY month
ORDER BY month`,
		}, ch4, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch4_chart", Name: "Monthly Revenue",
			SourceID: "lrn_ch4_q", ChartType: "lineY",
			XColumn: "month", YColumn: "revenue",
		}, ch4+chY, qY),

		// ── Ch 5: JOIN ────────────────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch5_md", Name: "Ch 5: JOIN",
			Content: "## Ch 5 — JOIN\n\n`JOIN` links rows from two tables on a shared key. The default `INNER JOIN` keeps only matching rows.\n\n**Try:**\n- Add `HAVING lifetime_value > 500`\n- Change `LIMIT 20` to see more customers\n- Add `c.email` to the SELECT list",
		}, ch5, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch5_q", Name: "Ch 5: Top Customers",
			SQL: `-- Top customers by lifetime spend (JOIN two tables)
SELECT
  c.name,
  c.state,
  COUNT(DISTINCT o.id)       AS orders,
  ROUND(SUM(o.total), 2)     AS lifetime_value
FROM lrn_orders o
JOIN lrn_customers c ON c.id = o.customer_id
WHERE o.status = 'completed'
GROUP BY c.id, c.name, c.state
ORDER BY lifetime_value DESC
LIMIT 20`,
		}, ch5, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch5_chart", Name: "Top 20 Customers",
			SourceID: "lrn_ch5_q", ChartType: "barY",
			XColumn: "name", YColumn: "lifetime_value", ColorColumn: "state",
		}, ch5+chY, qY),

		// ── Ch 6: Window Functions ────────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch6_md", Name: "Ch 6: Window Functions",
			Content: "## Ch 6 — Window Functions\n\n`OVER (ORDER BY ...)` runs a calculation across a sliding window of rows without collapsing them — unlike `GROUP BY`.\n\n**Try:**\n- Change `SUM` to `COUNT` for cumulative order count\n- Try `AVG(revenue) OVER (ORDER BY month ROWS BETWEEN 2 PRECEDING AND CURRENT ROW)` for a 3-month moving average\n- Add `DENSE_RANK() OVER (ORDER BY revenue DESC) AS rank`",
		}, ch6, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch6_q", Name: "Ch 6: Running Revenue",
			SQL: `-- Running revenue total and monthly rank using window functions
WITH monthly AS (
  SELECT
    STRFTIME(created_at, '%Y-%m') AS month,
    ROUND(SUM(total), 2)           AS revenue
  FROM lrn_orders
  WHERE status = 'completed'
  GROUP BY month
)
SELECT
  month,
  revenue,
  ROUND(SUM(revenue) OVER (ORDER BY month), 2) AS running_total,
  RANK() OVER (ORDER BY revenue DESC)          AS revenue_rank
FROM monthly
ORDER BY month`,
		}, ch6, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch6_chart", Name: "Cumulative Revenue",
			SourceID: "lrn_ch6_q", ChartType: "lineY",
			XColumn: "month", YColumn: "running_total",
		}, ch6+chY, qY),

		// ── Ch 7: CTEs + Subqueries ───────────────────────────────────────────
		at(CanvasAction{
			Type: "markdown", NodeID: "lrn_ch7_md", Name: "Ch 7: CTEs",
			Content: "## Ch 7 — CTEs & Subqueries\n\nA **CTE** (`WITH name AS (...)`) names a temporary result set for reuse — cleaner than nested subqueries.\n\n**Try:**\n- Change `>` to `>= 2 *` (double the average)\n- Add a second CTE for average orders per customer\n- Replace the final SELECT with `SELECT COUNT(*)` to count qualifying customers",
		}, ch7, 0),
		at(CanvasAction{
			Type: "query", NodeID: "lrn_ch7_q", Name: "Ch 7: High-Value Customers",
			SQL: `-- Find customers who spend more than average (using a CTE)
WITH customer_spend AS (
  SELECT
    c.id,
    c.name,
    c.state,
    ROUND(SUM(o.total), 2) AS total_spent
  FROM lrn_orders o
  JOIN lrn_customers c ON c.id = o.customer_id
  WHERE o.status = 'completed'
  GROUP BY c.id, c.name, c.state
),
avg_spend AS (
  SELECT ROUND(AVG(total_spent), 2) AS threshold FROM customer_spend
)
SELECT
  cs.name,
  cs.state,
  cs.total_spent,
  ROUND(cs.total_spent / avg_spend.threshold, 2) AS times_avg
FROM customer_spend cs, avg_spend
WHERE cs.total_spent > avg_spend.threshold
ORDER BY cs.total_spent DESC`,
		}, ch7, qY),
		at(CanvasAction{
			Type: "chart", NodeID: "lrn_ch7_chart", Name: "High-Value Customers",
			SourceID: "lrn_ch7_q", ChartType: "barY",
			XColumn: "name", YColumn: "total_spent", ColorColumn: "state",
		}, ch7+chY, qY),
	}, nil
}

// ── Weather ───────────────────────────────────────────────────────────────────

func (a *App) loadWeatherDataset() ([]CanvasAction, error) {
	if err := a.loadEmbeddedCSV("sampledata/weather/daily.csv", "wx_daily"); err != nil {
		return nil, err
	}
	a.SaveTableData("wx_daily") //nolint:errcheck

	const tempID = "demo_wx_temp"

	tempSQL := `SELECT date, city, temp_high_c
FROM wx_daily
ORDER BY date, city`

	monthlyTempSQL := `SELECT
    STRFTIME(date, '%Y-%m') AS month,
    city,
    ROUND(AVG(temp_high_c), 1) AS avg_high_c
FROM wx_daily
GROUP BY month, city
ORDER BY month, city`

	precipSQL := `SELECT
    STRFTIME(date, '%m') AS month_num,
    CASE STRFTIME(date, '%m')
        WHEN '01' THEN 'Jan' WHEN '02' THEN 'Feb' WHEN '03' THEN 'Mar'
        WHEN '04' THEN 'Apr' WHEN '05' THEN 'May' WHEN '06' THEN 'Jun'
        WHEN '07' THEN 'Jul' WHEN '08' THEN 'Aug' WHEN '09' THEN 'Sep'
        WHEN '10' THEN 'Oct' WHEN '11' THEN 'Nov' WHEN '12' THEN 'Dec'
    END AS month,
    city,
    ROUND(SUM(precipitation_mm), 1) AS total_mm
FROM wx_daily
GROUP BY month_num, month, city
ORDER BY month_num, city`

	condSQL := `SELECT condition, COUNT(*) AS days
FROM wx_daily
GROUP BY condition
ORDER BY days DESC
LIMIT 10`

	const (
		colA = 0.0
		colB = 360.0
		colC = 760.0
		tblY = 160.0
		sec1 = 420.0
		sec2 = 790.0
		mdH  = 100.0
	)

	return []CanvasAction{
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_wx_intro", Name: "wx_intro",
			Content: "# 🌤️ Global Weather\n\nDaily high/low temperature, precipitation, and sky conditions for five cities from 2021–2023. Notice Sydney's inverted seasons and Cairo's consistently low rainfall.",
		}, colA, 0),

		a.tableAction("demo_wx_tbl", "wx_daily", colA, tblY),

		at(CanvasAction{
			Type: "markdown", NodeID: "demo_wx_md1", Name: "wx_pipeline_label",
			Content: "## Query → Chart\nDaily temperature data sourced from one QueryNode, reused by the trend chart.",
		}, colA, sec1-mdH-10),
		at(CanvasAction{
			Type: "markdown", NodeID: "demo_wx_md2", Name: "wx_inline_label",
			Content: "## Inline SQL → Chart\nPrecipitation and conditions aggregated with inline SQL.",
		}, colC, sec1-mdH-10),

		at(CanvasAction{
			Type: "query", NodeID: tempID, Name: "Daily Temperature",
			SQL: tempSQL,
		}, colA, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_wx_temp_chart", Name: "High Temp by City",
			SourceID: tempID, ChartType: "lineY",
			XColumn: "date", YColumn: "temp_high_c", ColorColumn: "city",
		}, colB, sec1),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_wx_precip", Name: "Monthly Precipitation by City",
			SQL: precipSQL, ChartType: "barY",
			XColumn: "month", YColumn: "total_mm", ColorColumn: "city",
		}, colC, sec1),

		at(CanvasAction{
			Type: "query", NodeID: "demo_wx_monthly_q", Name: "Monthly Avg Temp",
			SQL: monthlyTempSQL,
		}, colA, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_wx_monthly_chart", Name: "Monthly Avg High °C",
			SourceID: "demo_wx_monthly_q", ChartType: "lineY",
			XColumn: "month", YColumn: "avg_high_c", ColorColumn: "city",
		}, colB, sec2),
		at(CanvasAction{
			Type: "chart", NodeID: "demo_wx_cond", Name: "Most Common Conditions",
			SQL: condSQL, ChartType: "barY",
			XColumn: "condition", YColumn: "days",
		}, colC, sec2),
	}, nil
}

// setMatchCheck builds an AssertionCheck of kind "set_match" for use in learn tracks.
func setMatchCheck(id, label, referenceSql, feedbackOnFail, hint string) map[string]interface{} {
	return map[string]interface{}{
		"id":             id,
		"kind":           "set_match",
		"label":          label,
		"referenceSql":   referenceSql,
		"feedbackOnFail": feedbackOnFail,
		"hint":           hint,
	}
}

func (a *App) loadLearnSQLBoltLesson1() ([]CanvasAction, error) {
	stmts := []string{
		`DROP TABLE IF EXISTS sb_movies`,
		`CREATE TABLE sb_movies AS SELECT * FROM (VALUES
  (1,  'Toy Story',             'John Lasseter',  1995, 81),
  (2,  'A Bug''s Life',         'John Lasseter',  1998, 95),
  (3,  'Toy Story 2',           'John Lasseter',  1999, 93),
  (4,  'Monsters, Inc.',        'Pete Docter',    2001, 92),
  (5,  'Finding Nemo',          'Andrew Stanton', 2003, 107),
  (6,  'The Incredibles',       'Brad Bird',      2004, 116),
  (7,  'Cars',                  'John Lasseter',  2006, 117),
  (8,  'Ratatouille',           'Brad Bird',      2007, 111),
  (9,  'WALL-E',                'Andrew Stanton', 2008, 104),
  (10, 'Up',                    'Pete Docter',    2009, 101),
  (11, 'Toy Story 3',           'Lee Unkrich',    2010, 103),
  (12, 'Cars 2',                'John Lasseter',  2011, 120),
  (13, 'Brave',                 'Brenda Chapman', 2012, 102),
  (14, 'Monsters University',   'Dan Scanlon',    2013, 110)
) t(id, title, director, year, length_minutes)`,
	}
	for _, s := range stmts {
		if _, err := a.duck.ExecContext(a.ctx, s); err != nil {
			return nil, fmt.Errorf("sqlbolt-lesson1 setup: %w", err)
		}
	}
	a.SaveTableData("sb_movies") //nolint:errcheck

	const (
		mdY  = 0.0   // intro markdown row
		tblY = 200.0 // table schema row
		asY  = 460.0 // assertion node row (single row — SQL editor embedded)
		colW = 760.0 // column width spacing (cards are 720px wide + 40px gap)
	)

	// Each exercise: markdown prompt, student query node, assertion node
	// Exercises are columns spaced colW apart
	exercises := []struct {
		col         float64
		id          string
		title       string
		prompt      string
		starterSQL  string
		refSQL      string
		checkLabel  string
		failMsg     string
		hint        string
		successText string
	}{
		{
			col:        0,
			id:         "sb_ex1",
			title:      "Exercise 1 — Titles only",
			prompt:     "## Exercise 1\n\nFind the **title** of each film in the `sb_movies` table.\n\n```sql\nSELECT ??? FROM sb_movies\n```",
			starterSQL: "SELECT ???\nFROM sb_movies",
			refSQL:     "SELECT title FROM sb_movies",
			checkLabel: "Returns only the title column",
			failMsg:    "Your result doesn't match. Make sure you SELECT just the `title` column from `sb_movies`.",
			hint:       "Try: `SELECT title FROM sb_movies`",
			successText: "Nice work! `SELECT column_name` is how you pick a single column. " +
				"Each column name maps to a field in the table — no quotes needed for column names.",
		},
		{
			col:        colW,
			id:         "sb_ex2",
			title:      "Exercise 2 — Directors only",
			prompt:     "## Exercise 2\n\nFind the **director** of each film.\n\n```sql\nSELECT ??? FROM sb_movies\n```",
			starterSQL: "SELECT ???\nFROM sb_movies",
			refSQL:     "SELECT director FROM sb_movies",
			checkLabel: "Returns only the director column",
			failMsg:    "Not quite. SELECT just the `director` column.",
			hint:       "Try: `SELECT director FROM sb_movies`",
			successText: "Exactly right! Same pattern, different column name. " +
				"You can `SELECT` any column that exists in the table — just use its name.",
		},
		{
			col:        colW * 2,
			id:         "sb_ex3",
			title:      "Exercise 3 — Title + Director",
			prompt:     "## Exercise 3\n\nFind the **title** and **director** of each film.\n\nSelect two columns, comma-separated.",
			starterSQL: "SELECT ???, ???\nFROM sb_movies",
			refSQL:     "SELECT title, director FROM sb_movies",
			checkLabel: "Returns title and director columns",
			failMsg:    "Check your column list. You need both `title` and `director`.",
			hint:       "Try: `SELECT title, director FROM sb_movies`",
			successText: "Great! Multiple columns are separated by commas. " +
				"The order you list them is the order they appear in the result — try swapping `director, title` to see.",
		},
		{
			col:        colW * 3,
			id:         "sb_ex4",
			title:      "Exercise 4 — Title + Year",
			prompt:     "## Exercise 4\n\nFind the **title** and **year** each film was released.",
			starterSQL: "SELECT ???, ???\nFROM sb_movies",
			refSQL:     "SELECT title, year FROM sb_movies",
			checkLabel: "Returns title and year columns",
			failMsg:    "You need `title` and `year`. Check column names in the schema above.",
			hint:       "Try: `SELECT title, year FROM sb_movies`",
			successText: "Well done! Notice `year` is a number — SQL returns it as-is. " +
				"One last exercise: what if you want *every* column without listing each one?",
		},
		{
			col:        colW * 4,
			id:         "sb_ex5",
			title:      "Exercise 5 — All columns",
			prompt:     "## Exercise 5\n\nFind **all the information** about each film.\n\nUse the wildcard shorthand instead of listing every column.",
			starterSQL: "SELECT ???\nFROM sb_movies",
			refSQL:     "SELECT * FROM sb_movies",
			checkLabel: "Returns all columns (wildcard)",
			failMsg:    "Use `SELECT *` to select every column at once.",
			hint:       "Try: `SELECT * FROM sb_movies`",
			successText: "🎉 Lesson complete! `SELECT *` is a quick way to grab everything — " +
				"great for exploration, though in production queries it's better to name the columns you need. " +
				"You now know the core of the `SELECT` statement. Head to Lesson 2 to learn filtering with `WHERE`.",
		},
	}

	actions := []CanvasAction{
		// Intro markdown + schema table, centered above exercises
		at(CanvasAction{
			Type: "markdown", NodeID: "sb_intro_md", Name: "Lesson 1: SELECT 101",
			Content: "# 🎬 SQLBolt Lesson 1 — SELECT Queries 101\n\nIn this lesson you'll practise the most fundamental SQL statement: **SELECT**.\n\nEach exercise is a single card with a **prompt**, a **SQL editor**, and **checks**. Edit the SQL, then click **Run** to check your answer.\n\nThe table you'll query is `sb_movies`, shown in the schema card to the right.\n\n> Exercises are chained left-to-right. Start with Exercise 1 and follow the **Next →** buttons.\n\n---\n\n> **Attribution** — These exercises are adapted from **[SQLBolt](https://sqlbolt.com)**, a free interactive SQL tutorial created by **Nick Ciubotariu**. The Movies dataset and exercise structure originate from SQLBolt and are reproduced here solely for educational demonstration within sql.garden.\n>\n> This sample is not affiliated with or endorsed by SQLBolt. For the full course — including `WHERE`, `JOIN`, aggregates, subqueries, and more — please visit **[sqlbolt.com](https://sqlbolt.com)**. It's excellent and completely free.",
		}, 0, mdY),
		a.tableAction("sb_movies_schema", "sb_movies", colW, mdY),
	}

	for i, ex := range exercises {
		nextID := ""
		if i < len(exercises)-1 {
			nextID = exercises[i+1].id + "_assert"
		}
		actions = append(actions,
			at(CanvasAction{
				Type:             "exercise",
				NodeID:           ex.id + "_assert",
				Name:             ex.title,
				SQL:              ex.starterSQL,
				Content:          ex.prompt,
				SuccessText:      ex.successText,
				NextID:           nextID,
				Checks:           []map[string]interface{}{setMatchCheck(ex.id+"_chk1", ex.checkLabel, ex.refSQL, ex.failMsg, ex.hint)},
				RevealHintsAfter: 2,
			}, ex.col, asY),
		)
	}

	return actions, nil
}
