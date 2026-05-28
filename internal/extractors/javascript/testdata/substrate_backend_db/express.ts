// Express backend-HTTP db_effect fixture (#2903).
// Hand-written, dependency-manifest-free. Each handler performs an ORM
// read and write so the jsts effect sniffer attributes db_read / db_write
// to the named handler function.
import express from "express";
import { User } from "./models";

const app = express();

export async function expressGetUser(req, res) {
  const user = await User.findOne({ where: { id: req.params.id } });
  res.json(user);
}

export async function expressCreateUser(req, res) {
  const created = await User.create({ name: req.body.name });
  res.status(201).json(created);
}

app.get("/users/:id", expressGetUser);
app.post("/users", expressCreateUser);
