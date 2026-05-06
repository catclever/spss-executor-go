export namespace main {
	
	export class AppConfig {
	    serverUrl: string;
	    spssPath: string;
	    llmModel: string;
	    apiKey: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverUrl = source["serverUrl"];
	        this.spssPath = source["spssPath"];
	        this.llmModel = source["llmModel"];
	        this.apiKey = source["apiKey"];
	    }
	}

}

