export namespace desktopapp {
	
	export class JobDTO {
	    id: string;
	    printer: string;
	    doc: string;
	    kind: string;
	    state: string;
	    percent: number;
	
	    static createFrom(source: any = {}) {
	        return new JobDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.printer = source["printer"];
	        this.doc = source["doc"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.percent = source["percent"];
	    }
	}
	export class JobHistoryDTO {
	    id: string;
	    printer: string;
	    doc: string;
	    kind: string;
	    status: string;
	    error: string;
	    finishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new JobHistoryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.printer = source["printer"];
	        this.doc = source["doc"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class PrinterDTO {
	    name: string;
	    model: string;
	    status: string;
	    stateReason: string;
	    connection: string;
	    location: string;
	    queued: number;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrinterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.model = source["model"];
	        this.status = source["status"];
	        this.stateReason = source["stateReason"];
	        this.connection = source["connection"];
	        this.location = source["location"];
	        this.queued = source["queued"];
	        this.default = source["default"];
	    }
	}
	export class StateDTO {
	    server: string;
	    host: string;
	    enrolled: boolean;
	    connected: boolean;
	    lastError: string;
	    version: string;
	    availableVersion: string;
	    lastCheck: string;
	    autoUpdate: boolean;
	    defaultPrinter: string;
	    theme: string;
	    notifyDone: boolean;
	    notifyFailed: boolean;
	    notifyWeekly: boolean;
	    jobsOk: number;
	    jobsFailed: number;
	    printers: PrinterDTO[];
	    activeJobs: JobDTO[];
	    recentJobs: JobHistoryDTO[];
	
	    static createFrom(source: any = {}) {
	        return new StateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = source["server"];
	        this.host = source["host"];
	        this.enrolled = source["enrolled"];
	        this.connected = source["connected"];
	        this.lastError = source["lastError"];
	        this.version = source["version"];
	        this.availableVersion = source["availableVersion"];
	        this.lastCheck = source["lastCheck"];
	        this.autoUpdate = source["autoUpdate"];
	        this.defaultPrinter = source["defaultPrinter"];
	        this.theme = source["theme"];
	        this.notifyDone = source["notifyDone"];
	        this.notifyFailed = source["notifyFailed"];
	        this.notifyWeekly = source["notifyWeekly"];
	        this.jobsOk = source["jobsOk"];
	        this.jobsFailed = source["jobsFailed"];
	        this.printers = this.convertValues(source["printers"], PrinterDTO);
	        this.activeJobs = this.convertValues(source["activeJobs"], JobDTO);
	        this.recentJobs = this.convertValues(source["recentJobs"], JobHistoryDTO);
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

