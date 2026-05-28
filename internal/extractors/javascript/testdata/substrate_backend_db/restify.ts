// Restify backend-HTTP db_effect fixture (#2903).
import restify from "restify";
import { User } from "./models";

const server = restify.createServer();

export async function restifyGetUser(req, res, next) {
  const user = await User.findOne({ where: { id: req.params.id } });
  res.send(user);
  return next();
}

export async function restifyCreateUser(req, res, next) {
  const created = await User.create({ name: req.body.name });
  res.send(201, created);
  return next();
}

server.get("/users/:id", restifyGetUser);
server.post("/users", restifyCreateUser);
