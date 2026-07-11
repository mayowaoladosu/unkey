console.log(
  JSON.stringify({
    level: "info",
    message: "cron execution",
    resource: process.env.LAYER_RAIL_RESOURCE_ID,
    time: new Date().toISOString(),
  }),
);
