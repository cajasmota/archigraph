// Polka backend-HTTP db_effect fixture (#2903).
import polka from "polka";
import { User } from "./models";

export async function polkaGetUser(req, res) {
  const user = await User.findOne({ where: { id: req.params.id } });
  res.end(JSON.stringify(user));
}

export async function polkaCreateUser(req, res) {
  const created = await User.create({ name: req.body.name });
  res.end(JSON.stringify(created));
}

polka().get("/users/:id", polkaGetUser).post("/users", polkaCreateUser);
