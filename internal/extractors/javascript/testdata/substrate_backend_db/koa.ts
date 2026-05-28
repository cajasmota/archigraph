// Koa backend-HTTP db_effect fixture (#2903).
import Koa from "koa";
import Router from "@koa/router";
import { User } from "./models";

const app = new Koa();
const router = new Router();

export async function koaGetUser(ctx) {
  ctx.body = await User.findOne({ where: { id: ctx.params.id } });
}

export async function koaCreateUser(ctx) {
  ctx.body = await User.create({ name: ctx.request.body.name });
}

router.get("/users/:id", koaGetUser);
router.post("/users", koaCreateUser);
app.use(router.routes());
