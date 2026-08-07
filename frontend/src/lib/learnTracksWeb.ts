// Web-side learn track data. Mirrors sampledata.go but runs entirely in DuckDB WASM.
// tableAction() nodes (which need Go-side column introspection) are replaced with
// query preview nodes — equally useful for learners.

export interface WebLearnTrack {
  id: string
  name: string
  description: string
  icon: string
  level: string
  chapters: number
  tables: string[]
  rowCount: number
  tags: string[]
}

export const WEB_LEARN_TRACKS: WebLearnTrack[] = [
  {
    id: 'ecommerce-sql',
    name: 'SQL Fundamentals: E-Commerce',
    description: 'Learn SQL from SELECT to CTEs using a realistic store dataset: 100 customers, 40 products, ~500 orders, and ~1,100 order items across 8 interactive chapters.',
    icon: '🛍️',
    level: 'Beginner',
    chapters: 8,
    tables: ['lrn_customers', 'lrn_products', 'lrn_orders', 'lrn_order_items'],
    rowCount: 1740,
    tags: ['SELECT', 'WHERE', 'GROUP BY', 'JOIN', 'Window Functions', 'CTE'],
  },
  {
    id: 'sqlbolt-lesson1',
    name: 'SQLBolt Lesson 1: SELECT 101',
    description: '5 interactive SELECT exercises adapted from SQLBolt (sqlbolt.com) by Nick Ciubotariu, included here for educational demonstration. For the full free course visit sqlbolt.com.',
    icon: '🎬',
    level: 'Beginner',
    chapters: 1,
    tables: ['sb_movies'],
    rowCount: 14,
    tags: ['SELECT', 'columns', 'wildcard'],
  },
]

// ── Data-generation SQL ───────────────────────────────────────────────────────

const ECOMMERCE_SQL = [
  `DROP TABLE IF EXISTS lrn_order_items`,
  `DROP TABLE IF EXISTS lrn_orders`,
  `DROP TABLE IF EXISTS lrn_customers`,
  `DROP TABLE IF EXISTS lrn_products`,

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

  `UPDATE lrn_orders SET total=(
  SELECT COALESCE(ROUND(SUM(quantity*unit_price),2),0)
  FROM lrn_order_items WHERE order_id=lrn_orders.id
)`,
]

const SQLBOLT_SQL = [
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
]

// ── Canvas action builders ────────────────────────────────────────────────────

type Action = Record<string, unknown>

function at(action: Action, x: number, y: number): Action {
  return { ...action, x, y, hasPosition: true }
}

function tablePreview(nodeId: string, tableName: string, x: number, y: number): Action {
  return at({
    type: 'query',
    nodeId,
    name: tableName,
    sql: `SELECT * FROM ${tableName} LIMIT 20`,
  }, x, y)
}

function setMatchCheck(id: string, label: string, referenceSql: string, feedbackOnFail: string, hint: string) {
  return { id, kind: 'set_match', label, referenceSql, feedbackOnFail, hint }
}

// ── Ecommerce track actions ───────────────────────────────────────────────────

const ch0 = 0, ch1 = 1000, ch2 = 1800, ch3 = 2600
const ch4 = 3400, ch5 = 4200, ch6 = 5000, ch7 = 5800
const qY = 240, chY = 300

function ecommerceActions(): Action[] {
  return [
    at({ type: 'markdown', nodeId: 'lrn_intro_md', name: 'intro',
      content: '# 🛍️ SQL Fundamentals: E-Commerce\n\nLearn SQL by exploring a realistic store: **100 customers**, **40 products**, **~500 orders**, and **~1,100 order items** generated fresh in your local DuckDB.\n\nScroll right through **8 chapters** — each introduces a new SQL concept. Run the query, read the results, then try the exercises in the comments.\n\n**Tables:** `lrn_customers` · `lrn_products` · `lrn_orders` · `lrn_order_items`',
    }, ch0, 0),

    tablePreview('lrn_tbl_customers',   'lrn_customers',    ch0,       qY),
    tablePreview('lrn_tbl_products',    'lrn_products',     ch0 + 205, qY),
    tablePreview('lrn_tbl_orders',      'lrn_orders',       ch0 + 410, qY),
    tablePreview('lrn_tbl_items',       'lrn_order_items',  ch0 + 615, qY),

    at({ type: 'query', nodeId: 'lrn_setup_q', name: 'Dataset Summary',
      sql: `SELECT 'lrn_customers'    AS "table", COUNT(*) AS rows FROM lrn_customers
UNION ALL SELECT 'lrn_products',    COUNT(*) FROM lrn_products
UNION ALL SELECT 'lrn_orders',      COUNT(*) FROM lrn_orders
UNION ALL SELECT 'lrn_order_items', COUNT(*) FROM lrn_order_items`,
    }, ch0, qY + 200),
    at({ type: 'chart', nodeId: 'lrn_setup_chart', name: 'Row counts',
      sourceId: 'lrn_setup_q', chartType: 'barY', xColumn: 'table', yColumn: 'rows',
    }, ch0 + chY, qY + 200),

    at({ type: 'markdown', nodeId: 'lrn_ch1_md', name: 'Ch 1: SELECT',
      content: '## Ch 1 — SELECT\n\n`SELECT` retrieves columns from a table. `ORDER BY` sorts rows; `*` selects all columns.\n\n**Try:**\n- Sort by `price ASC` instead\n- Replace the column list with `*`\n- Add `LIMIT 5` at the end',
    }, ch1, 0),
    at({ type: 'query', nodeId: 'lrn_ch1_q', name: 'Ch 1: Products by Category',
      sql: `-- Select product name, category, and price, sorted by category then price
SELECT name, category, price
FROM lrn_products
ORDER BY category, price DESC`,
    }, ch1, qY),
    at({ type: 'chart', nodeId: 'lrn_ch1_chart', name: 'Product Prices',
      sourceId: 'lrn_ch1_q', chartType: 'barY', xColumn: 'name', yColumn: 'price', colorColumn: 'category',
    }, ch1 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch2_md', name: 'Ch 2: WHERE',
      content: '## Ch 2 — WHERE\n\n`WHERE` filters rows before returning them. Combine conditions with `AND` / `OR`.\n\n**Try:**\n- Change `\'Electronics\'` to `\'Books\'`\n- Replace `< 100` with `BETWEEN 50 AND 200`\n- Remove the category filter entirely',
    }, ch2, 0),
    at({ type: 'query', nodeId: 'lrn_ch2_q', name: 'Ch 2: Budget Electronics',
      sql: `-- Find Electronics products under $100
SELECT name, price
FROM lrn_products
WHERE category = 'Electronics'
  AND price < 100
ORDER BY price ASC`,
    }, ch2, qY),
    at({ type: 'chart', nodeId: 'lrn_ch2_chart', name: 'Budget Electronics',
      sourceId: 'lrn_ch2_q', chartType: 'barY', xColumn: 'name', yColumn: 'price',
    }, ch2 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch3_md', name: 'Ch 3: ORDER BY & LIMIT',
      content: '## Ch 3 — ORDER BY & LIMIT\n\n`ORDER BY col DESC` sorts highest-first. `LIMIT N` caps the row count — useful for top-N reports.\n\n**Try:**\n- Change `DESC` to `ASC` (cheapest first)\n- Try `LIMIT 5` or `LIMIT 20`\n- Sort by `name` alphabetically',
    }, ch3, 0),
    at({ type: 'query', nodeId: 'lrn_ch3_q', name: 'Ch 3: Top 10 by Price',
      sql: `-- Top 10 most expensive products
SELECT name, category, price
FROM lrn_products
ORDER BY price DESC
LIMIT 10`,
    }, ch3, qY),
    at({ type: 'chart', nodeId: 'lrn_ch3_chart', name: 'Top 10 Priciest Products',
      sourceId: 'lrn_ch3_q', chartType: 'barY', xColumn: 'name', yColumn: 'price', colorColumn: 'category',
    }, ch3 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch4_md', name: 'Ch 4: GROUP BY',
      content: '## Ch 4 — GROUP BY\n\n`GROUP BY` collapses rows into groups. Aggregates (`COUNT`, `SUM`, `AVG`, `MIN`, `MAX`) summarise each group.\n\n**Try:**\n- Group by `status` instead of month\n- Replace `SUM` with `AVG` for average order value\n- Add `HAVING revenue > 2000` after `GROUP BY`',
    }, ch4, 0),
    at({ type: 'query', nodeId: 'lrn_ch4_q', name: 'Ch 4: Monthly Revenue',
      sql: `-- Orders and revenue by month
SELECT
  STRFTIME(created_at, '%Y-%m') AS month,
  COUNT(*)                       AS total_orders,
  ROUND(SUM(total), 2)           AS revenue
FROM lrn_orders
WHERE status = 'completed'
GROUP BY month
ORDER BY month`,
    }, ch4, qY),
    at({ type: 'chart', nodeId: 'lrn_ch4_chart', name: 'Monthly Revenue',
      sourceId: 'lrn_ch4_q', chartType: 'lineY', xColumn: 'month', yColumn: 'revenue',
    }, ch4 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch5_md', name: 'Ch 5: JOIN',
      content: '## Ch 5 — JOIN\n\n`JOIN` links rows from two tables on a shared key. The default `INNER JOIN` keeps only matching rows.\n\n**Try:**\n- Add `HAVING lifetime_value > 500`\n- Change `LIMIT 20` to see more customers\n- Add `c.email` to the SELECT list',
    }, ch5, 0),
    at({ type: 'query', nodeId: 'lrn_ch5_q', name: 'Ch 5: Top Customers',
      sql: `-- Top customers by lifetime spend (JOIN two tables)
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
    at({ type: 'chart', nodeId: 'lrn_ch5_chart', name: 'Top 20 Customers',
      sourceId: 'lrn_ch5_q', chartType: 'barY', xColumn: 'name', yColumn: 'lifetime_value', colorColumn: 'state',
    }, ch5 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch6_md', name: 'Ch 6: Window Functions',
      content: '## Ch 6 — Window Functions\n\n`OVER (ORDER BY ...)` runs a calculation across a sliding window of rows without collapsing them — unlike `GROUP BY`.\n\n**Try:**\n- Change `SUM` to `COUNT` for cumulative order count\n- Try `AVG(revenue) OVER (ORDER BY month ROWS BETWEEN 2 PRECEDING AND CURRENT ROW)` for a 3-month moving average\n- Add `DENSE_RANK() OVER (ORDER BY revenue DESC) AS rank`',
    }, ch6, 0),
    at({ type: 'query', nodeId: 'lrn_ch6_q', name: 'Ch 6: Running Revenue',
      sql: `-- Running revenue total and monthly rank using window functions
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
    at({ type: 'chart', nodeId: 'lrn_ch6_chart', name: 'Cumulative Revenue',
      sourceId: 'lrn_ch6_q', chartType: 'lineY', xColumn: 'month', yColumn: 'running_total',
    }, ch6 + chY, qY),

    at({ type: 'markdown', nodeId: 'lrn_ch7_md', name: 'Ch 7: CTEs',
      content: '## Ch 7 — CTEs & Subqueries\n\nA **CTE** (`WITH name AS (...)`) names a temporary result set for reuse — cleaner than nested subqueries.\n\n**Try:**\n- Change `>` to `>= 2 *` (double the average)\n- Add a second CTE for average orders per customer\n- Replace the final SELECT with `SELECT COUNT(*)` to count qualifying customers',
    }, ch7, 0),
    at({ type: 'query', nodeId: 'lrn_ch7_q', name: 'Ch 7: High-Value Customers',
      sql: `-- Find customers who spend more than average (using a CTE)
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
    at({ type: 'chart', nodeId: 'lrn_ch7_chart', name: 'High-Value Customers',
      sourceId: 'lrn_ch7_q', chartType: 'barY', xColumn: 'name', yColumn: 'total_spent', colorColumn: 'state',
    }, ch7 + chY, qY),
  ]
}

// ── SQLBolt track actions ─────────────────────────────────────────────────────

function sqlboltActions(): Action[] {
  const colW = 760
  const mdY = 0, asY = 460

  const exercises = [
    {
      col: 0, id: 'sb_ex1', title: 'Exercise 1 — Titles only',
      prompt: '## Exercise 1\n\nFind the **title** of each film in the `sb_movies` table.\n\n```sql\nSELECT ??? FROM sb_movies\n```',
      starterSQL: 'SELECT ???\nFROM sb_movies',
      refSQL: 'SELECT title FROM sb_movies',
      checkLabel: 'Returns only the title column',
      failMsg: 'Your result doesn\'t match. Make sure you SELECT just the `title` column from `sb_movies`.',
      hint: 'Try: `SELECT title FROM sb_movies`',
      successText: 'Nice work! `SELECT column_name` is how you pick a single column. Each column name maps to a field in the table — no quotes needed for column names.',
    },
    {
      col: colW, id: 'sb_ex2', title: 'Exercise 2 — Directors only',
      prompt: '## Exercise 2\n\nFind the **director** of each film.\n\n```sql\nSELECT ??? FROM sb_movies\n```',
      starterSQL: 'SELECT ???\nFROM sb_movies',
      refSQL: 'SELECT director FROM sb_movies',
      checkLabel: 'Returns only the director column',
      failMsg: 'Not quite. SELECT just the `director` column.',
      hint: 'Try: `SELECT director FROM sb_movies`',
      successText: 'Exactly right! Same pattern, different column name. You can `SELECT` any column that exists in the table — just use its name.',
    },
    {
      col: colW * 2, id: 'sb_ex3', title: 'Exercise 3 — Title + Director',
      prompt: '## Exercise 3\n\nFind the **title** and **director** of each film.\n\nSelect two columns, comma-separated.',
      starterSQL: 'SELECT ???, ???\nFROM sb_movies',
      refSQL: 'SELECT title, director FROM sb_movies',
      checkLabel: 'Returns title and director columns',
      failMsg: 'Check your column list. You need both `title` and `director`.',
      hint: 'Try: `SELECT title, director FROM sb_movies`',
      successText: 'Great! Multiple columns are separated by commas. The order you list them is the order they appear in the result — try swapping `director, title` to see.',
    },
    {
      col: colW * 3, id: 'sb_ex4', title: 'Exercise 4 — Title + Year',
      prompt: '## Exercise 4\n\nFind the **title** and **year** each film was released.',
      starterSQL: 'SELECT ???, ???\nFROM sb_movies',
      refSQL: 'SELECT title, year FROM sb_movies',
      checkLabel: 'Returns title and year columns',
      failMsg: 'You need `title` and `year`. Check column names in the schema above.',
      hint: 'Try: `SELECT title, year FROM sb_movies`',
      successText: 'Well done! Notice `year` is a number — SQL returns it as-is. One last exercise: what if you want *every* column without listing each one?',
    },
    {
      col: colW * 4, id: 'sb_ex5', title: 'Exercise 5 — All columns',
      prompt: '## Exercise 5\n\nFind **all the information** about each film.\n\nUse the wildcard shorthand instead of listing every column.',
      starterSQL: 'SELECT ???\nFROM sb_movies',
      refSQL: 'SELECT * FROM sb_movies',
      checkLabel: 'Returns all columns (wildcard)',
      failMsg: 'Use `SELECT *` to select every column at once.',
      hint: 'Try: `SELECT * FROM sb_movies`',
      successText: '🎉 Lesson complete! `SELECT *` is a quick way to grab everything — great for exploration, though in production queries it\'s better to name the columns you need. You now know the core of the `SELECT` statement. Head to Lesson 2 to learn filtering with `WHERE`.',
    },
  ]

  const actions: Action[] = [
    at({ type: 'markdown', nodeId: 'sb_intro_md', name: 'Lesson 1: SELECT 101',
      content: '# 🎬 SQLBolt Lesson 1 — SELECT Queries 101\n\nIn this lesson you\'ll practise the most fundamental SQL statement: **SELECT**.\n\nEach exercise is a single card with a **prompt**, a **SQL editor**, and **checks**. Edit the SQL, then click **Run** to check your answer.\n\nThe table you\'ll query is `sb_movies`, shown in the schema card to the right.\n\n> Exercises are chained left-to-right. Start with Exercise 1 and follow the **Next →** buttons.\n\n---\n\n> **Attribution** — These exercises are adapted from **[SQLBolt](https://sqlbolt.com)**, a free interactive SQL tutorial created by **Nick Ciubotariu**. The Movies dataset and exercise structure originate from SQLBolt and are reproduced here solely for educational demonstration within sql.garden.\n>\n> This sample is not affiliated with or endorsed by SQLBolt. For the full course — including `WHERE`, `JOIN`, aggregates, subqueries, and more — please visit **[sqlbolt.com](https://sqlbolt.com)**. It\'s excellent and completely free.',
    }, 0, mdY),
    tablePreview('sb_movies_schema', 'sb_movies', colW, mdY),
  ]

  exercises.forEach((ex, i) => {
    const nextID = i < exercises.length - 1 ? exercises[i + 1].id + '_assert' : ''
    actions.push(at({
      type: 'exercise',
      nodeId: ex.id + '_assert',
      name: ex.title,
      sql: ex.starterSQL,
      content: ex.prompt,
      successText: ex.successText,
      nextId: nextID || undefined,
      checks: [setMatchCheck(ex.id + '_chk1', ex.checkLabel, ex.refSQL, ex.failMsg, ex.hint)],
      revealHintsAfter: 2,
    }, ex.col, asY))
  })

  return actions
}

// ── Public API ────────────────────────────────────────────────────────────────

export function getWebLearnTrackSQL(id: string): string[] {
  if (id === 'ecommerce-sql') return ECOMMERCE_SQL
  if (id === 'sqlbolt-lesson1') return SQLBOLT_SQL
  return []
}

export function getWebLearnTrackActions(id: string): Action[] {
  if (id === 'ecommerce-sql') return ecommerceActions()
  if (id === 'sqlbolt-lesson1') return sqlboltActions()
  return []
}
