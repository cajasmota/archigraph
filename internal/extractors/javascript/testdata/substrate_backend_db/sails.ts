// Sails backend-HTTP db_effect fixture (#2903).
// Sails actions call Waterline model methods (find / create).

export async function sailsFindUsers(req, res) {
  const users = await User.find({ active: true });
  return res.json(users);
}

export async function sailsCreateUser(req, res) {
  const created = await User.create(req.body).fetch();
  return res.status(201).json(created);
}
