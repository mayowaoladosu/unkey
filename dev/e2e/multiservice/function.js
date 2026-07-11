exports.handler = async (event) => ({
  statusCode: 200,
  headers: { "content-type": "application/json" },
  body: {
    service: "function",
    method: event.method,
    resourceId: process.env.LAYER_RAIL_RESOURCE_ID,
  },
});
