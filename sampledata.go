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

	volatilitySQL := `SELECT
    ticker,
    ROUND(STDDEV(close / LAG(close) OVER (PARTITION BY ticker ORDER BY date) - 1) * SQRT(252) * 100, 1) AS annual_vol_pct
FROM mkt_prices
ORDER BY annual_vol_pct DESC NULLS LAST
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
