# playground

> The goal of this page is to provide a simple DuckDB editor experience. Allow the upload or paste of raw data (or even web data) and run queries and reports on it.

## Examples

```js
import { convertCsv } from './helpers/helpers.js';

import { DuckDBClient } from "npm:@observablehq/duckdb";
import { Editor } from "./components/Editor.js";
```

<div class="size-clamp">

### CSV Data Loader

> Paste csv data with the first row having the columns

```js
const csv = view(Editor({ value: `id,name,month
1,tony,jan
2,bb,feb
3,pp,mar
4,ducky,apr` }))
```

```js
display(d3)
const d3Parsed = await d3.csvParse(csv);
```
</div>

```js
const blob = new Blob([csv], { type: "text/csv" });
const objectURL = URL.createObjectURL(blob);
// display(objectURL)
```

```js emit
const rawCsv = (await (await fetch(objectURL)).text())

// display("Raw CSV")
// display(rawCsv)

const data = rawCsv.split('\n').map(e => e.split(','))
// display("Data")

// display(convertCsv(JSON.parse(JSON.stringify(data))))

const db = await DuckDBClient.of({ csv: await d3.csvParse(csv, d3.autoType) })
// display(db)
// display(await db.describeTables());

```
### Query Explorer

<div class="editor-container">

<div class="size-clamp">

#### Editor
```js
const query = view(Editor({ value: `SELECT * FROM csv` }))
```
</div>

<div>

#### Result

```js
Inputs.table(await db.query(query))
```
</div>

</div>



<style>
.editor-container {
        max-width: 960px;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        gap: 10px;
}

.editor-container > * {
        width: 50%;
        flex: 0 1 1fr;
}

.size-clamp {
        max-height: 300px;
        overflow-y: scroll;
}
</style>


## Appendix

- built in `d3` methods help parse and autoType the columns passed in
- Flow:
        1. Editor component for raw table and sql
        2. `welcome.md` has an example of using raw SQL Schemas
        3. This has support for raw csv data and parsing

Currently working on 2 flows: 1: paste CSV and query it, 2: Define a schema, and Query it (make the process of adding data simpler)

Need to figure out what the north star solution is.


### Notes

- [Data schema validation](https://observablehq.com/@observablehq/database-client-specification#%C2%A72.2)
- [import and parse a dataset](https://observablehq.com/@pablotheurer/import-and-parse-your-dataset/2)
- [d3.csvParse](https://d3js.org/d3-dsv#csvParse)
- [d3.autoType](https://d3js.org/d3-dsv#autoType)
- [Datasettes](https://github.com/MainakRepositor/Datasets/tree/master)