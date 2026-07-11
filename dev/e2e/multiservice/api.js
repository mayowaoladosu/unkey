const http = require("node:http");

http
  .createServer((request, response) => {
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify({ service: "api", path: request.url, healthy: true }));
  })
  .listen(Number(process.env.PORT || 8081), "0.0.0.0", () => {
    console.log(JSON.stringify({ level: "info", message: "private api ready", resource: process.env.LAYER_RAIL_RESOURCE_ID }));
  });
