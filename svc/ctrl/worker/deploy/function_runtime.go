package deploy

import (
	"fmt"
	"strings"
)

const nodeFunctionAdapter = `const http=require("http"),path=require("path"),{pathToFileURL}=require("url");
const spec=process.env.LAYER_RAIL_HANDLER||"";const cut=spec.lastIndexOf(".");if(cut<1)throw new Error("LAYER_RAIL_HANDLER must be module.export");
const modulePath=spec.slice(0,cut),exportName=spec.slice(cut+1);let loaded;
async function handler(){if(loaded)return loaded;let last;for(const candidate of [modulePath,modulePath+".js",modulePath+".mjs",modulePath+".cjs"]){try{const mod=await import(pathToFileURL(path.resolve(candidate)).href);loaded=mod[exportName]||(mod.default&&mod.default[exportName])||(exportName==="default"&&mod.default);if(typeof loaded==="function")return loaded;}catch(err){last=err;}}throw last||new Error("function handler not found");}
http.createServer(async(req,res)=>{try{const chunks=[];let size=0;for await(const chunk of req){size+=chunk.length;if(size>6291456)throw Object.assign(new Error("request body too large"),{statusCode:413});chunks.push(chunk);}const raw=Buffer.concat(chunks);const url=new URL(req.url,"http://function.local");const event={method:req.method,path:url.pathname,query:Object.fromEntries(url.searchParams),headers:req.headers,body:raw.toString("utf8"),isBase64Encoded:false};const out=await (await handler())(event,{requestId:req.headers["x-request-id"]||crypto.randomUUID()});if(out instanceof Response){res.statusCode=out.status;out.headers.forEach((v,k)=>res.setHeader(k,v));res.end(Buffer.from(await out.arrayBuffer()));return;}const value=out&&typeof out==="object"?out:{body:out};res.statusCode=value.statusCode||200;for(const [k,v] of Object.entries(value.headers||{}))res.setHeader(k,String(v));let body=value.body??"";if(value.isBase64Encoded)body=Buffer.from(String(body),"base64");else if(typeof body!=="string"&&!Buffer.isBuffer(body)){res.setHeader("content-type","application/json");body=JSON.stringify(body);}res.end(body);}catch(err){res.statusCode=err.statusCode||500;res.setHeader("content-type","application/json");res.end(JSON.stringify({error:"function invocation failed",message:String(err.message||err)}));}}).listen(Number(process.env.PORT||8080),"0.0.0.0");`

const pythonFunctionAdapter = `import os,json,base64,importlib
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
spec=os.environ.get("LAYER_RAIL_HANDLER","")
module_name,sep,export_name=spec.rpartition(".")
if not sep: raise RuntimeError("LAYER_RAIL_HANDLER must be module.function")
module_name=module_name.replace("/",".").removesuffix(".py")
handler=getattr(importlib.import_module(module_name),export_name)
class Server(BaseHTTPRequestHandler):
 def invoke(self):
  try:
   length=int(self.headers.get("content-length","0")); raw=self.rfile.read(length) if length else b""
   event={"method":self.command,"path":self.path.split("?",1)[0],"headers":dict(self.headers),"body":raw.decode("utf-8"),"isBase64Encoded":False}
   out=handler(event,{"requestId":self.headers.get("x-request-id","")})
   value=out if isinstance(out,dict) else {"body":out}; body=value.get("body","")
   if value.get("isBase64Encoded"): body=base64.b64decode(str(body))
   elif not isinstance(body,(str,bytes)): body=json.dumps(body); value.setdefault("headers",{})["content-type"]="application/json"
   if isinstance(body,str): body=body.encode()
   self.send_response(value.get("statusCode",200))
   for key,val in value.get("headers",{}).items(): self.send_header(key,str(val))
   self.send_header("content-length",str(len(body))); self.end_headers(); self.wfile.write(body)
  except Exception as err:
   body=json.dumps({"error":"function invocation failed","message":str(err)}).encode(); self.send_response(500); self.send_header("content-type","application/json"); self.send_header("content-length",str(len(body))); self.end_headers(); self.wfile.write(body)
 do_GET=do_POST=do_PUT=do_PATCH=do_DELETE=do_OPTIONS=invoke
 def log_message(self,format,*args): print(format%args,flush=True)
ThreadingHTTPServer(("0.0.0.0",int(os.environ.get("PORT","8080"))),Server).serve_forever()`

func functionRuntimeCommand(runtime string) ([]string, error) {
	normalized := strings.ToLower(strings.TrimSpace(runtime))
	switch {
	case normalized == "node", strings.HasPrefix(normalized, "nodejs"):
		return []string{"node", "-e", nodeFunctionAdapter}, nil
	case normalized == "python", strings.HasPrefix(normalized, "python3"):
		return []string{"python3", "-c", pythonFunctionAdapter}, nil
	default:
		return nil, fmt.Errorf("unsupported function runtime %q", runtime)
	}
}
