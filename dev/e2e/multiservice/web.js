const http = require("node:http");

http
  .createServer(async (_request, response) => {
    try {
      const [apiResponse, functionResponse] = await Promise.all([
        fetch(`${process.env.API_URL}/health`).then((result) => result.json()),
        fetch(`${process.env.FUNCTION_URL}/invoke`).then((result) => result.json()),
      ]);
      response.setHeader("content-type", "application/json");
      response.end(
        JSON.stringify({
          service: "web",
          resourceId: process.env.LAYER_RAIL_RESOURCE_ID,
          api: apiResponse,
          function: functionResponse,
        }),
      );
    } catch (error) {
      response.statusCode = 503;
      response.end(JSON.stringify({ error: String(error) }));
    }
  })
  .listen(Number(process.env.PORT || 8080), "0.0.0.0", () => {
    console.log(JSON.stringify({ level: "info", message: "public web ready", resource: process.env.LAYER_RAIL_RESOURCE_ID }));
  });
