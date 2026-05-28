// Fastify backend-HTTP db_effect fixture (#2903).
import Fastify from "fastify";
import { prisma } from "./db";

const app = Fastify();

export async function fastifyGetUser(req, reply) {
  const user = await prisma.user.findUnique({ where: { id: req.params.id } });
  reply.send(user);
}

export async function fastifyCreateUser(req, reply) {
  const created = await prisma.user.create({ data: req.body });
  reply.code(201).send(created);
}

app.get("/users/:id", fastifyGetUser);
app.post("/users", fastifyCreateUser);
