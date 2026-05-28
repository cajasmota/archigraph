// Hono backend-HTTP db_effect fixture (#2903).
import { Hono } from "hono";
import { db } from "./db";

const app = new Hono();

export async function honoGetUser(c) {
  const user = await db.user.findFirst({ where: { id: c.req.param("id") } });
  return c.json(user);
}

export async function honoCreateUser(c) {
  const body = await c.req.json();
  const created = await db.user.create({ data: body });
  return c.json(created, 201);
}

app.get("/users/:id", honoGetUser);
app.post("/users", honoCreateUser);
