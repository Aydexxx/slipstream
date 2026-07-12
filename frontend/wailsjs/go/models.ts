export namespace fastmode {
	
	export class Status {
	    state: string;
	    mode: string;
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

}

export namespace privatemode {
	
	export class Status {
	    state: string;
	    endpoint: string;
	    hasConfig: boolean;
	    // Go type: time
	    lastHandshake: any;
	    handshakeAgeSec: number;
	    attempt: number;
	    rxBytes: number;
	    txBytes: number;
	    error: string;
	    // Go type: time
	    since: any;
	    killSwitchArmed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.endpoint = source["endpoint"];
	        this.hasConfig = source["hasConfig"];
	        this.lastHandshake = this.convertValues(source["lastHandshake"], null);
	        this.handshakeAgeSec = source["handshakeAgeSec"];
	        this.attempt = source["attempt"];
	        this.rxBytes = source["rxBytes"];
	        this.txBytes = source["txBytes"];
	        this.error = source["error"];
	        this.since = this.convertValues(source["since"], null);
	        this.killSwitchArmed = source["killSwitchArmed"];
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
	export class Summary {
	    endpoint: string;
	    endpointHost: string;
	    dns: string[];
	    addresses: string[];
	    fullTunnel: boolean;
	    obfuscated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.endpointHost = source["endpointHost"];
	        this.dns = source["dns"];
	        this.addresses = source["addresses"];
	        this.fullTunnel = source["fullTunnel"];
	        this.obfuscated = source["obfuscated"];
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
	    privateStatus?: privatemode.Status;
	    killSwitchArmed: boolean;
	    reconnectOnLaunch: boolean;
	
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
	        this.privateStatus = this.convertValues(source["privateStatus"], privatemode.Status);
	        this.killSwitchArmed = source["killSwitchArmed"];
	        this.reconnectOnLaunch = source["reconnectOnLaunch"];
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

