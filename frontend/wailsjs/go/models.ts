export namespace fastmode {
	
	export class Status {
	    state: string;
	    mode: string;
	    strategy: string;
	    domains: string[];
	    pid: number;
	    restarts: number;
	    error: string;
	    dnsApplied: boolean;
	    // Go type: time
	    since: any;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.mode = source["mode"];
	        this.strategy = source["strategy"];
	        this.domains = source["domains"];
	        this.pid = source["pid"];
	        this.restarts = source["restarts"];
	        this.error = source["error"];
	        this.dnsApplied = source["dnsApplied"];
	        this.since = this.convertValues(source["since"], null);
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
	export class StrategyInfo {
	    id: string;
	    name: string;
	    group: string;
	    description: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StrategyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.description = source["description"];
	        this.default = source["default"];
	    }
	}

}

export namespace statemachine {
	
	export class Status {
	    state: string;
	    subMode: string;
	    transitioning: boolean;
	    healthy: boolean;
	    error: string;
	    // Go type: time
	    since: any;
	    fastStatus?: fastmode.Status;
	    reconnectOnLaunch: boolean;
	    lastFastStrategy: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.subMode = source["subMode"];
	        this.transitioning = source["transitioning"];
	        this.healthy = source["healthy"];
	        this.error = source["error"];
	        this.since = this.convertValues(source["since"], null);
	        this.fastStatus = this.convertValues(source["fastStatus"], fastmode.Status);
	        this.reconnectOnLaunch = source["reconnectOnLaunch"];
	        this.lastFastStrategy = source["lastFastStrategy"];
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

}

