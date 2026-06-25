import { serve } from "@hono/node-server";
import { Hono } from "hono";
import { devices } from "../macAddress";
import * as wol from "wake_on_lan";

const app = new Hono();

app.get("/", (c) => {
	return c.text("Hello Hono!");
});

app.get("/invoke", (c) => {
	console.log("invoked!");
	return c.text("invoked!");
});

app.get("/devices", (c) => {
	return c.json(devices);
});

app.get("/wake", async (c) => {
	const name = c.req.query("device");
	if (!name) {
		return c.json({ error: "Missing ?device= query param", devices: devices.map((d) => d.name) }, 400);
	}

	const device = devices.find((d) => d.name.toLowerCase() === name.toLowerCase());
	if (!device) {
		return c.json({ error: `Unknown device: "${name}"`, devices: devices.map((d) => d.name) }, 404);
	}

	const result = await new Promise<"ok" | string>((resolve) => {
		wol.wake(device.mac, (error) => {
			if (error) {
				resolve(error.message ?? String(error));
			} else {
				resolve("ok");
			}
		});
	});

	const info = {
		device: device.name,
		mac: device.mac,
		status: result === "ok" ? "sent" : "failed",
		...(result !== "ok" && { error: result }),
	};

	console.log(`[WOL] ${device.name} (${device.mac}) → ${info.status}`);

	return c.json(info);
});

const port = 3000;
console.log(`Server is running on port ${port}`);

serve({
	fetch: app.fetch,
	port,
});
