// Feathers backend-HTTP db_effect fixture (#2903).
// A Feathers service delegates to a Mongoose model for persistence.
import { UserModel } from "./models";

export async function feathersFindUsers(params) {
  return await UserModel.find(params.query);
}

export async function feathersCreateUser(data) {
  return await UserModel.create(data);
}
