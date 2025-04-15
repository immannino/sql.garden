---
toc: false
---

# welcome


## TODO

CREATE SCHEMA

SELECT FORM SCHEMA
UDPATE SHIT
GET ALL DATA WoRKING
> sql garden is an omage to a simpler time, a time when we sowed our own schemas and grew our databases organically.

```js echo
import { DuckDBClient } from "npm:@observablehq/duckdb";
import { Editor } from "./components/Editor.js";

const db = DuckDBClient.of({});
display(db)
// const conn = db.connect();
```

```js
const schema = view(Editor({value: `CREATE TABLE IF NOT EXISTS foo (name TEXT NOT NULL, added TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP );

INSERT INTO foo (name) VALUES
  ('Tony'),
  ('PP'),
  ('BB'),
  ('Ducky'),
  ('Kellie');

SELECT * FROM foo;
`
}));
```

### Query Explorer
<div style="max-width: 960px; display: flex; flex-direction: row; justify-content: space-between; gap: 10px;">

<div style="width: 50%; flex: 0 1 50%;">

#### Editor
```js
const query = view(Editor({ value: `INSERT INTO foo (name) 
VALUES 
('tony');` }))
```
</div>

<div style="width: 50%; flex: 0 1 50%;">

#### Result

```js
Inputs.table(await db.query(query))
```
</div>

</div>

```js echo
display(await db.query(schema))
```

#### tables
```js echo
audit;
const tables = await db.describeTables()
```

```js echo
const audit = view(Inputs.button("audit"));
```
```js echo
display(tables)
```

```js echo
audit;
// run;
const count = await db.query(`SELECT * FROM foo ORDER BY added DESC`)
Inputs.table(count)
```

<!-- ```js
const clear = view(Inputs.button("reset"));
```

```js
clear;
await db.sql`DROP TABLE foo`
``` -->
<!-- 
```js echo
const q = await db.query(query)
``` -->

<!-- ```js
display(q)
```

```js echo
// eval(input)
input
```

```js echo
```

```js echo
await db.query(input)
``` -->

<style>
import url('npm:prismjs/themes/prism-tomorrow.css');
</style>