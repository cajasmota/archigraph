// AdonisJS backend-HTTP db_effect fixture (#2903).
// Lucid ORM models expose findOrFail / create off the model class.
import User from "App/Models/User";

export default class UsersController {
  async adonisShow({ params }) {
    return await User.findOne({ id: params.id });
  }

  async adonisStore({ request }) {
    return await User.create(request.only(["name"]));
  }
}
