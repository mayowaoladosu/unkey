async function tick() {
  const response = await fetch(`${process.env.API_URL}/worker`).then((result) => result.json());
  console.log(JSON.stringify({ level: "info", message: "worker tick", api: response.service, resource: process.env.LAYER_RAIL_RESOURCE_ID }));
}

tick().catch(console.error);
setInterval(() => tick().catch(console.error), 10_000);
