import { useDuckDB } from '../composables/useDuckDB'
import type { main } from '../../wailsjs/go/models'

export interface WebDataset {
  id: string
  name: string
  description: string
  icon: string
  tables: string[]
  rowCount: number
}

export const WEB_DATASETS: WebDataset[] = [
  {
    id: 'f1',
    name: 'F1 2023 Season',
    description: 'Driver standings, race results, and team performance across all 22 rounds of the 2023 Formula 1 World Championship.',
    icon: '🏎️',
    tables: ['f1_drivers', 'f1_teams', 'f1_races', 'f1_results'],
    rowCount: 492,
  },
  {
    id: 'markets',
    name: 'Stock Markets',
    description: 'Daily OHLCV price data for 10 tickers — AAPL, MSFT, GOOGL, AMZN, TSLA, NVDA and more — from January 2020 through December 2024.',
    icon: '📈',
    tables: ['mkt_companies', 'mkt_prices'],
    rowCount: 13050,
  },
  {
    id: 'economy',
    name: 'Macroeconomics',
    description: 'Monthly US economic indicators from 2014–2024: GDP growth, unemployment, CPI inflation, Fed funds rate, and S&P 500.',
    icon: '🏛️',
    tables: ['eco_indicators'],
    rowCount: 132,
  },
  {
    id: 'ecommerce',
    name: 'E-commerce',
    description: 'A small online store: 10 users, 10 products, 66 orders, and 175 line items across 12 months. Great for exploring JOIN patterns.',
    icon: '🛒',
    tables: ['ec_users', 'ec_products', 'ec_orders', 'ec_items'],
    rowCount: 261,
  },
  {
    id: 'weather',
    name: 'Global Weather',
    description: 'Daily high/low temperature, precipitation, and conditions for New York, London, Tokyo, Sydney, and Cairo from 2021–2023.',
    icon: '🌤️',
    tables: ['wx_daily'],
    rowCount: 5475,
  },
]

// ── Helpers ───────────────────────────────────────────────────────────────────

async function fetchAndLoad(csvPath: string, tableName: string): Promise<void> {
  const { exec, registerFile, dropFile } = useDuckDB()
  const resp = await fetch(csvPath)
  if (!resp.ok) throw new Error(`Failed to fetch ${csvPath}: HTTP ${resp.status}`)
  const buffer = new Uint8Array(await resp.arrayBuffer())
  const fname = `__sd_${tableName}.csv`
  await registerFile(fname, buffer)
  try {
    await exec(`DROP TABLE IF EXISTS "${tableName}"`)
    await exec(`CREATE TABLE "${tableName}" AS SELECT * FROM read_csv_auto('${fname}', header=true)`)
  } finally {
    await dropFile(fname)
  }
}

async function tableAction(nodeId: string, tableName: string, x: number, y: number): Promise<main.CanvasAction> {
  const { query, getTableInfo } = useDuckDB()
  const [cols, countResult] = await Promise.all([
    getTableInfo(tableName),
    query(`SELECT COUNT(*) AS n FROM "${tableName}"`),
  ])
  const rowCount = Number(countResult.rows[0]?.n ?? 0)
  return {
    type: 'table',
    nodeId,
    name: tableName,
    columns: cols.map((c) => ({ name: c.name, type: c.type })),
    rowCount,
    hasPosition: true,
    x,
    y,
  } as main.CanvasAction
}

function at(action: Partial<main.CanvasAction>, x: number, y: number): main.CanvasAction {
  return { ...action, hasPosition: true, x, y } as main.CanvasAction
}

// ── Dataset loaders ───────────────────────────────────────────────────────────

async function loadF1(): Promise<main.CanvasAction[]> {
  const base = import.meta.env.BASE_URL + 'sampledata/f1/'
  await Promise.all([
    fetchAndLoad(base + 'drivers.csv', 'f1_drivers'),
    fetchAndLoad(base + 'teams.csv', 'f1_teams'),
    fetchAndLoad(base + 'races.csv', 'f1_races'),
    fetchAndLoad(base + 'results.csv', 'f1_results'),
  ])

  const standingsID = 'demo_f1_standings'
  const progressionID = 'demo_f1_progression'

  const standingsSQL = `SELECT d.name AS driver, d.team AS team, SUM(r.points) AS total_points, COUNT(CASE WHEN r.position = 1 THEN 1 END) AS wins, COUNT(CASE WHEN TRY_CAST(r.position AS INT) <= 3 THEN 1 END) AS podiums FROM f1_results r JOIN f1_drivers d ON r.driver_id = d.driver_id WHERE r.status = 'Finished' OR r.points > 0 GROUP BY d.driver_id, d.name, d.team ORDER BY total_points DESC`
  const progressionSQL = `WITH top5 AS (SELECT d.driver_id FROM f1_results r JOIN f1_drivers d ON r.driver_id = d.driver_id GROUP BY d.driver_id ORDER BY SUM(r.points) DESC LIMIT 5) SELECT ra.round, d.name AS driver, SUM(r.points) OVER (PARTITION BY d.driver_id ORDER BY ra.round) AS cumulative_points FROM f1_results r JOIN f1_drivers d ON r.driver_id = d.driver_id JOIN f1_races ra ON r.race_id = ra.race_id WHERE d.driver_id IN (SELECT driver_id FROM top5) ORDER BY ra.round, cumulative_points DESC`
  const constructorSQL = `SELECT d.team AS constructor, SUM(r.points) AS points FROM f1_results r JOIN f1_drivers d ON r.driver_id = d.driver_id GROUP BY d.team ORDER BY points DESC`
  const winsSQL = `SELECT d.name AS driver, COUNT(*) AS wins FROM f1_results r JOIN f1_drivers d ON r.driver_id = d.driver_id WHERE r.position = 1 GROUP BY d.driver_id, d.name ORDER BY wins DESC`

  const colA = 0, colB = 360, colC = 760, tblY = 160, sec1 = 420, sec2 = 790, mdH = 100

  return [
    at({ type: 'markdown', nodeId: 'demo_f1_intro', name: 'f1_intro', content: '# 🏎️ F1 2023 Season\n\nFormula 1 race results across all 22 rounds of the 2023 World Championship. Explore driver standings, team performance, and points progression using **QueryNodes** that feed data into **Charts** — or write SQL directly inside a chart.' }, colA, 0),
    await tableAction('demo_f1_tbl_drivers', 'f1_drivers', colA, tblY),
    await tableAction('demo_f1_tbl_teams', 'f1_teams', 250, tblY),
    await tableAction('demo_f1_tbl_races', 'f1_races', 500, tblY),
    await tableAction('demo_f1_tbl_results', 'f1_results', 740, tblY),
    at({ type: 'markdown', nodeId: 'demo_f1_md1', name: 'f1_pipeline_label', content: '## Query → Chart\nA **QueryNode** runs SQL and caches results. A **ChartNode** can source from it — no duplicate queries.' }, colA, sec1 - mdH - 10),
    at({ type: 'markdown', nodeId: 'demo_f1_md2', name: 'f1_inline_label', content: '## Inline SQL → Chart\nA ChartNode can also run its own SQL directly — handy for simple one-off visualisations.' }, colC, sec1 - mdH - 10),
    at({ type: 'query', nodeId: standingsID, name: 'Driver Standings', sql: standingsSQL }, colA, sec1),
    at({ type: 'chart', nodeId: 'demo_f1_standings_chart', name: 'Championship Points', sourceId: standingsID, chartType: 'barY', xColumn: 'driver', yColumn: 'total_points' }, colB, sec1),
    at({ type: 'chart', nodeId: 'demo_f1_constructors', name: 'Constructor Standings', sql: constructorSQL, chartType: 'barY', xColumn: 'constructor', yColumn: 'points' }, colC, sec1),
    at({ type: 'query', nodeId: progressionID, name: 'Points Progression (Top 5)', sql: progressionSQL }, colA, sec2),
    at({ type: 'chart', nodeId: 'demo_f1_progression_chart', name: 'Season Points Race', sourceId: progressionID, chartType: 'lineY', xColumn: 'round', yColumn: 'cumulative_points', colorColumn: 'driver' }, colB, sec2),
    at({ type: 'chart', nodeId: 'demo_f1_wins', name: 'Race Wins', sql: winsSQL, chartType: 'barY', xColumn: 'driver', yColumn: 'wins' }, colC, sec2),
  ]
}

async function loadMarkets(): Promise<main.CanvasAction[]> {
  const base = import.meta.env.BASE_URL + 'sampledata/markets/'
  await Promise.all([
    fetchAndLoad(base + 'companies.csv', 'mkt_companies'),
    fetchAndLoad(base + 'prices.csv', 'mkt_prices'),
  ])

  const histID = 'demo_mkt_history'
  const histSQL = `SELECT date, ticker, close FROM mkt_prices ORDER BY date, ticker`
  const returnsSQL = `WITH first_last AS (SELECT ticker, FIRST(close ORDER BY date) AS price_start, LAST(close ORDER BY date) AS price_end FROM mkt_prices GROUP BY ticker) SELECT c.name AS company, c.sector, f.ticker, ROUND((f.price_end - f.price_start) / f.price_start * 100, 1) AS total_return_pct FROM first_last f JOIN mkt_companies c ON f.ticker = c.ticker ORDER BY total_return_pct DESC`
  const volumeSQL = `SELECT ticker, ROUND(AVG(volume) / 1e6, 2) AS avg_daily_vol_m FROM mkt_prices GROUP BY ticker ORDER BY avg_daily_vol_m DESC`
  const volatilitySQL = `SELECT ticker, ROUND(STDDEV(close / LAG(close) OVER (PARTITION BY ticker ORDER BY date) - 1) * SQRT(252) * 100, 1) AS annual_vol_pct FROM mkt_prices ORDER BY annual_vol_pct DESC NULLS LAST LIMIT 10`

  const colA = 0, colB = 360, colC = 760, tblY = 160, sec1 = 420, sec2 = 790, mdH = 100

  return [
    at({ type: 'markdown', nodeId: 'demo_mkt_intro', name: 'mkt_intro', content: '# 📈 Stock Markets\n\nDaily OHLCV data for 10 tickers from January 2020 through December 2024. Explore price history, total returns, volatility, and trading volume with DuckDB\'s window functions.' }, colA, 0),
    await tableAction('demo_mkt_tbl_co', 'mkt_companies', colA, tblY),
    await tableAction('demo_mkt_tbl_px', 'mkt_prices', 260, tblY),
    at({ type: 'markdown', nodeId: 'demo_mkt_md1', name: 'mkt_pipeline_label', content: '## Query → Chart\nPrice history sourced from a QueryNode — change the SQL once, all charts update.' }, colA, sec1 - mdH - 10),
    at({ type: 'markdown', nodeId: 'demo_mkt_md2', name: 'mkt_inline_label', content: '## Inline SQL → Chart\nQuick one-off charts with SQL written directly in the node.' }, colC, sec1 - mdH - 10),
    at({ type: 'query', nodeId: histID, name: 'Price History', sql: histSQL }, colA, sec1),
    at({ type: 'chart', nodeId: 'demo_mkt_hist_chart', name: 'Close Price by Ticker', sourceId: histID, chartType: 'lineY', xColumn: 'date', yColumn: 'close', colorColumn: 'ticker' }, colB, sec1),
    at({ type: 'chart', nodeId: 'demo_mkt_returns', name: 'Total Return %', sql: returnsSQL, chartType: 'barY', xColumn: 'ticker', yColumn: 'total_return_pct', colorColumn: 'sector' }, colC, sec1),
    at({ type: 'query', nodeId: 'demo_mkt_vol_query', name: 'Volatility', sql: volatilitySQL }, colA, sec2),
    at({ type: 'chart', nodeId: 'demo_mkt_vol_chart', name: 'Annualised Volatility %', sourceId: 'demo_mkt_vol_query', chartType: 'barY', xColumn: 'ticker', yColumn: 'annual_vol_pct' }, colB, sec2),
    at({ type: 'chart', nodeId: 'demo_mkt_avgvol', name: 'Avg Daily Volume (M)', sql: volumeSQL, chartType: 'barY', xColumn: 'ticker', yColumn: 'avg_daily_vol_m' }, colC, sec2),
  ]
}

async function loadEconomy(): Promise<main.CanvasAction[]> {
  await fetchAndLoad(import.meta.env.BASE_URL + 'sampledata/economy/indicators.csv', 'eco_indicators')

  const ecoID = 'demo_eco_all'
  const allSQL = `SELECT date, gdp_growth_pct, unemployment_pct, cpi_yoy_pct, fed_funds_rate, sp500_close FROM eco_indicators ORDER BY date`
  const cpiSQL = `SELECT date, cpi_yoy_pct FROM eco_indicators ORDER BY date`
  const ffrSQL = `SELECT date, fed_funds_rate FROM eco_indicators ORDER BY date`
  const gdpSQL = `SELECT STRFTIME(date, '%Y') AS year, ROUND(AVG(gdp_growth_pct), 2) AS avg_gdp_growth FROM eco_indicators GROUP BY year ORDER BY year`

  const colA = 0, colB = 360, colC = 760, tblY = 160, sec1 = 420, sec2 = 790, mdH = 100

  return [
    at({ type: 'markdown', nodeId: 'demo_eco_intro', name: 'eco_intro', content: '# 🏛️ US Macroeconomics\n\nMonthly indicators from 2014–2024. Watch how the Fed funds rate responded to the 2020 COVID shock and 2022 inflation surge. GDP, unemployment, CPI, and S&P 500 all in one table.' }, colA, 0),
    await tableAction('demo_eco_tbl', 'eco_indicators', colA, tblY),
    at({ type: 'markdown', nodeId: 'demo_eco_md1', name: 'eco_pipeline_label', content: '## Query → Chart\nOne QueryNode fetches everything; individual charts slice different columns from the same result.' }, colA, sec1 - mdH - 10),
    at({ type: 'markdown', nodeId: 'demo_eco_md2', name: 'eco_inline_label', content: '## Inline SQL → Chart\nFed funds rate and GDP aggregated with inline SQL — no shared QueryNode needed.' }, colC, sec1 - mdH - 10),
    at({ type: 'query', nodeId: ecoID, name: 'All Indicators', sql: allSQL }, colA, sec1),
    at({ type: 'chart', nodeId: 'demo_eco_unemp', name: 'Unemployment Rate %', sourceId: ecoID, chartType: 'lineY', xColumn: 'date', yColumn: 'unemployment_pct' }, colB, sec1),
    at({ type: 'chart', nodeId: 'demo_eco_cpi_inline', name: 'CPI Inflation YoY %', sql: cpiSQL, chartType: 'lineY', xColumn: 'date', yColumn: 'cpi_yoy_pct' }, colC, sec1),
    at({ type: 'query', nodeId: 'demo_eco_ffr_query', name: 'Fed Funds Rate', sql: ffrSQL }, colA, sec2),
    at({ type: 'chart', nodeId: 'demo_eco_ffr_chart', name: 'Fed Funds Rate %', sourceId: 'demo_eco_ffr_query', chartType: 'areaY', xColumn: 'date', yColumn: 'fed_funds_rate' }, colB, sec2),
    at({ type: 'chart', nodeId: 'demo_eco_gdp', name: 'Avg GDP Growth by Year', sql: gdpSQL, chartType: 'barY', xColumn: 'year', yColumn: 'avg_gdp_growth' }, colC, sec2),
  ]
}

async function loadEcommerce(): Promise<main.CanvasAction[]> {
  const base = import.meta.env.BASE_URL + 'sampledata/ecommerce/'
  await Promise.all([
    fetchAndLoad(base + 'users.csv', 'ec_users'),
    fetchAndLoad(base + 'products.csv', 'ec_products'),
    fetchAndLoad(base + 'orders.csv', 'ec_orders'),
    fetchAndLoad(base + 'items.csv', 'ec_items'),
  ])

  const revenueID = 'demo_ec_revenue'
  const revenueSQL = `SELECT p.name AS product, p.category, SUM(i.quantity * i.unit_price) AS revenue, SUM(i.quantity) AS units_sold FROM ec_items i JOIN ec_products p ON i.product_id = p.product_id GROUP BY p.product_id, p.name, p.category ORDER BY revenue DESC`
  const monthlySQL = `SELECT STRFTIME(o.created_at, '%Y-%m') AS month, COUNT(DISTINCT o.order_id) AS orders, ROUND(SUM(o.total), 2) AS revenue FROM ec_orders o WHERE o.status != 'cancelled' GROUP BY month ORDER BY month`
  const ltvSQL = `SELECT u.name, u.plan, COUNT(DISTINCT o.order_id) AS orders, ROUND(SUM(o.total), 2) AS lifetime_value FROM ec_orders o JOIN ec_users u ON o.user_id = u.user_id WHERE o.status != 'cancelled' GROUP BY u.user_id, u.name, u.plan ORDER BY lifetime_value DESC`
  const categorySQL = `SELECT p.category, ROUND(SUM(i.quantity * i.unit_price), 2) AS revenue FROM ec_items i JOIN ec_products p ON i.product_id = p.product_id GROUP BY p.category ORDER BY revenue DESC`

  const colA = 0, colB = 360, colC = 760, tblY = 160, sec1 = 420, sec2 = 790, mdH = 100

  return [
    at({ type: 'markdown', nodeId: 'demo_ec_intro', name: 'ec_intro', content: '# 🛒 E-commerce\n\nA small online store with users, products, orders, and line items across 12 months. Great for practising multi-table JOINs, GROUP BY aggregations, and revenue analysis.' }, colA, 0),
    await tableAction('demo_ec_tbl_u', 'ec_users', colA, tblY),
    await tableAction('demo_ec_tbl_p', 'ec_products', 240, tblY),
    await tableAction('demo_ec_tbl_o', 'ec_orders', 480, tblY),
    await tableAction('demo_ec_tbl_i', 'ec_items', 720, tblY),
    at({ type: 'markdown', nodeId: 'demo_ec_md1', name: 'ec_pipeline_label', content: '## Query → Chart\nRevenue by product sourced from a shared QueryNode that also powers the units-sold view.' }, colA, sec1 - mdH - 10),
    at({ type: 'markdown', nodeId: 'demo_ec_md2', name: 'ec_inline_label', content: '## Inline SQL → Chart\nCategory breakdown and monthly trend written directly in each chart.' }, colC, sec1 - mdH - 10),
    at({ type: 'query', nodeId: revenueID, name: 'Revenue by Product', sql: revenueSQL }, colA, sec1),
    at({ type: 'chart', nodeId: 'demo_ec_rev_chart', name: 'Product Revenue', sourceId: revenueID, chartType: 'barY', xColumn: 'product', yColumn: 'revenue', colorColumn: 'category' }, colB, sec1),
    at({ type: 'chart', nodeId: 'demo_ec_cat', name: 'Revenue by Category', sql: categorySQL, chartType: 'barY', xColumn: 'category', yColumn: 'revenue' }, colC, sec1),
    at({ type: 'query', nodeId: 'demo_ec_monthly_q', name: 'Monthly Revenue', sql: monthlySQL }, colA, sec2),
    at({ type: 'chart', nodeId: 'demo_ec_monthly_chart', name: 'Monthly Revenue Trend', sourceId: 'demo_ec_monthly_q', chartType: 'lineY', xColumn: 'month', yColumn: 'revenue' }, colB, sec2),
    at({ type: 'chart', nodeId: 'demo_ec_ltv', name: 'Customer Lifetime Value', sql: ltvSQL, chartType: 'barY', xColumn: 'name', yColumn: 'lifetime_value', colorColumn: 'plan' }, colC, sec2),
  ]
}

async function loadWeather(): Promise<main.CanvasAction[]> {
  await fetchAndLoad(import.meta.env.BASE_URL + 'sampledata/weather/daily.csv', 'wx_daily')

  const tempID = 'demo_wx_temp'
  const tempSQL = `SELECT date, city, temp_high_c FROM wx_daily ORDER BY date, city`
  const monthlyTempSQL = `SELECT STRFTIME(date, '%Y-%m') AS month, city, ROUND(AVG(temp_high_c), 1) AS avg_high_c FROM wx_daily GROUP BY month, city ORDER BY month, city`
  const precipSQL = `SELECT STRFTIME(date, '%m') AS month_num, CASE STRFTIME(date, '%m') WHEN '01' THEN 'Jan' WHEN '02' THEN 'Feb' WHEN '03' THEN 'Mar' WHEN '04' THEN 'Apr' WHEN '05' THEN 'May' WHEN '06' THEN 'Jun' WHEN '07' THEN 'Jul' WHEN '08' THEN 'Aug' WHEN '09' THEN 'Sep' WHEN '10' THEN 'Oct' WHEN '11' THEN 'Nov' WHEN '12' THEN 'Dec' END AS month, city, ROUND(SUM(precipitation_mm), 1) AS total_mm FROM wx_daily GROUP BY month_num, month, city ORDER BY month_num, city`
  const condSQL = `SELECT condition, COUNT(*) AS days FROM wx_daily GROUP BY condition ORDER BY days DESC LIMIT 10`

  const colA = 0, colB = 360, colC = 760, tblY = 160, sec1 = 420, sec2 = 790, mdH = 100

  return [
    at({ type: 'markdown', nodeId: 'demo_wx_intro', name: 'wx_intro', content: '# 🌤️ Global Weather\n\nDaily high/low temperature, precipitation, and sky conditions for five cities from 2021–2023. Notice Sydney\'s inverted seasons and Cairo\'s consistently low rainfall.' }, colA, 0),
    await tableAction('demo_wx_tbl', 'wx_daily', colA, tblY),
    at({ type: 'markdown', nodeId: 'demo_wx_md1', name: 'wx_pipeline_label', content: '## Query → Chart\nDaily temperature data sourced from one QueryNode, reused by the trend chart.' }, colA, sec1 - mdH - 10),
    at({ type: 'markdown', nodeId: 'demo_wx_md2', name: 'wx_inline_label', content: '## Inline SQL → Chart\nPrecipitation and conditions aggregated with inline SQL.' }, colC, sec1 - mdH - 10),
    at({ type: 'query', nodeId: tempID, name: 'Daily Temperature', sql: tempSQL }, colA, sec1),
    at({ type: 'chart', nodeId: 'demo_wx_temp_chart', name: 'High Temp by City', sourceId: tempID, chartType: 'lineY', xColumn: 'date', yColumn: 'temp_high_c', colorColumn: 'city' }, colB, sec1),
    at({ type: 'chart', nodeId: 'demo_wx_precip', name: 'Monthly Precipitation by City', sql: precipSQL, chartType: 'barY', xColumn: 'month', yColumn: 'total_mm', colorColumn: 'city' }, colC, sec1),
    at({ type: 'query', nodeId: 'demo_wx_monthly_q', name: 'Monthly Avg Temp', sql: monthlyTempSQL }, colA, sec2),
    at({ type: 'chart', nodeId: 'demo_wx_monthly_chart', name: 'Monthly Avg High °C', sourceId: 'demo_wx_monthly_q', chartType: 'lineY', xColumn: 'month', yColumn: 'avg_high_c', colorColumn: 'city' }, colB, sec2),
    at({ type: 'chart', nodeId: 'demo_wx_cond', name: 'Most Common Conditions', sql: condSQL, chartType: 'barY', xColumn: 'condition', yColumn: 'days' }, colC, sec2),
  ]
}

export async function loadWebDataset(id: string): Promise<main.CanvasAction[]> {
  switch (id) {
    case 'f1': return loadF1()
    case 'markets': return loadMarkets()
    case 'economy': return loadEconomy()
    case 'ecommerce': return loadEcommerce()
    case 'weather': return loadWeather()
    default: throw new Error(`Unknown dataset: ${id}`)
  }
}
