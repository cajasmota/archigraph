// Marble.js backend-HTTP db_effect fixture (#2903).
// Marble effects are RxJS pipelines; the handler still issues ORM calls.
import { User } from "./models";

export async function marbleGetUser(id) {
  return await User.findOne({ where: { id } });
}

export async function marbleCreateUser(payload) {
  return await User.create(payload);
}
