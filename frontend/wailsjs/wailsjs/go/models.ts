export namespace desktopapp {
	
	export class DeviceDTO {
	    uri: string;
	    name: string;
	    info: string;
	    makeModel: string;
	    connection: string;
	    driver: string;
	    installed: boolean;
	    queue: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.name = source["name"];
	        this.info = source["info"];
	        this.makeModel = source["makeModel"];
	        this.connection = source["connection"];
	        this.driver = source["driver"];
	        this.installed = source["installed"];
	        this.queue = source["queue"];
	    }
	}
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
	export class PreviewDTO {
	    dataUrl: string;
	    width: number;
	    height: number;
	    srcWidth: number;
	    srcHeight: number;
	    format: string;
	    printable: boolean;
	    image: boolean;
	    note: string;
	    raw: boolean;
	    tearOffMm: number;
	    tearOffPx: number;
	    lengthMm: number;
	
	    static createFrom(source: any = {}) {
	        return new PreviewDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataUrl = source["dataUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.srcWidth = source["srcWidth"];
	        this.srcHeight = source["srcHeight"];
	        this.format = source["format"];
	        this.printable = source["printable"];
	        this.image = source["image"];
	        this.note = source["note"];
	        this.raw = source["raw"];
	        this.tearOffMm = source["tearOffMm"];
	        this.tearOffPx = source["tearOffPx"];
	        this.lengthMm = source["lengthMm"];
	    }
	}
	export class PrintOptionsDTO {
	    path: string;
	    printer: string;
	    mode: string;
	    dither: boolean;
	    threshold: number;
	    rotate: number;
	    invert: boolean;
	    widthPx: number;
	    copies: number;
	    cut: boolean;
	    tearOffMm: number;
	    align: number;
	    media: string;
	    fitToPage: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintOptionsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.printer = source["printer"];
	        this.mode = source["mode"];
	        this.dither = source["dither"];
	        this.threshold = source["threshold"];
	        this.rotate = source["rotate"];
	        this.invert = source["invert"];
	        this.widthPx = source["widthPx"];
	        this.copies = source["copies"];
	        this.cut = source["cut"];
	        this.tearOffMm = source["tearOffMm"];
	        this.align = source["align"];
	        this.media = source["media"];
	        this.fitToPage = source["fitToPage"];
	    }
	}
	export class PrinterDTO {
	    name: string;
	    model: string;
	    raw: boolean;
	    status: string;
	    stateReason: string;
	    connection: string;
	    location: string;
	    queued: number;
	    default: boolean;
	    placeholder: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrinterDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.model = source["model"];
	        this.raw = source["raw"];
	        this.status = source["status"];
	        this.stateReason = source["stateReason"];
	        this.connection = source["connection"];
	        this.location = source["location"];
	        this.queued = source["queued"];
	        this.default = source["default"];
	        this.placeholder = source["placeholder"];
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

