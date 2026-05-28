// hapi backend-HTTP db_effect fixture (#2903).
import Hapi from "@hapi/hapi";
import { User } from "./models";

export async function hapiGetUser(request, h) {
  const user = await User.findOne({ where: { id: request.params.id } });
  return h.response(user);
}

export async function hapiCreateUser(request, h) {
  const created = await User.create({ name: request.payload.name });
  return h.response(created).code(201);
}

const server = Hapi.server({ port: 3000 });
server.route({ method: "GET", path: "/users/{id}", handler: hapiGetUser });
server.route({ method: "POST", path: "/users", handler: hapiCreateUser });
