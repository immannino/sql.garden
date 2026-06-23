export namespace main {
	
	export class AIChatMessage {
	    role: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new AIChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class CanvasColumn {
	    name: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new CanvasColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	    }
	}
	export class CanvasAction {
	    type: string;
	    nodeId?: string;
	    name: string;
	    sql?: string;
	    content?: string;
	    sourceId?: string;
	    chartType?: string;
	    xColumn?: string;
	    yColumn?: string;
	    colorColumn?: string;
	    labelColumn?: string;
	    columns?: CanvasColumn[];
	    rowCount?: number;
	    hasPosition?: boolean;
	    x?: number;
	    y?: number;
	
	    static createFrom(source: any = {}) {
	        return new CanvasAction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.nodeId = source["nodeId"];
	        this.name = source["name"];
	        this.sql = source["sql"];
	        this.content = source["content"];
	        this.sourceId = source["sourceId"];
	        this.chartType = source["chartType"];
	        this.xColumn = source["xColumn"];
	        this.yColumn = source["yColumn"];
	        this.colorColumn = source["colorColumn"];
	        this.labelColumn = source["labelColumn"];
	        this.columns = this.convertValues(source["columns"], CanvasColumn);
	        this.rowCount = source["rowCount"];
	        this.hasPosition = source["hasPosition"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIResponse {
	    content: string;
	    canvasActions: CanvasAction[];
	
	    static createFrom(source: any = {}) {
	        return new AIResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.canvasActions = this.convertValues(source["canvasActions"], CanvasAction);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AISettings {
	    enabled: boolean;
	    provider: string;
	    apiKey: string;
	    model: string;
	    userPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new AISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.userPrompt = source["userPrompt"];
	    }
	}
	export class AppSettings {
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	    }
	}
	export class AttachOptions {
	    alias: string;
	    dsn: string;
	    type: string;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AttachOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alias = source["alias"];
	        this.dsn = source["dsn"];
	        this.type = source["type"];
	        this.readOnly = source["readOnly"];
	    }
	}
	export class AutoConnectResult {
	    id: string;
	    name: string;
	    alias: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AutoConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.alias = source["alias"];
	        this.error = source["error"];
	    }
	}
	
	
	export class ColumnInfo {
	    name: string;
	    type: string;
	    primaryKey: boolean;
	    nullable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.primaryKey = source["primaryKey"];
	        this.nullable = source["nullable"];
	    }
	}
	export class ColumnMeta {
	    name: string;
	    type: string;
	    nullable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	    }
	}
	export class ConnectionRecord {
	    id: string;
	    name: string;
	    type: string;
	    dsn: string;
	    readOnly: boolean;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.dsn = source["dsn"];
	        this.readOnly = source["readOnly"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class MCPConfigStatus {
	    found: boolean;
	    path: string;
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCPConfigStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.path = source["path"];
	        this.configured = source["configured"];
	    }
	}
	export class QueryResult {
	    columns: string[];
	    columnTypes: string[];
	    rows: any[];
	    rowCount: number;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.columnTypes = source["columnTypes"];
	        this.rows = source["rows"];
	        this.rowCount = source["rowCount"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class SampleDataset {
	    id: string;
	    name: string;
	    description: string;
	    icon: string;
	    tables: string[];
	    rowCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SampleDataset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.icon = source["icon"];
	        this.tables = source["tables"];
	        this.rowCount = source["rowCount"];
	    }
	}
	export class SchemaInfo {
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SchemaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	    }
	}
	export class TableInfo {
	    name: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new TableInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	    }
	}
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	    releaseURL: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	        this.releaseURL = source["releaseURL"];
	    }
	}

}

