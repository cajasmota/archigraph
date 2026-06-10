import { Request, Response } from "express";
import { db } from "../db";
import { logger } from "../logger";
import { HttpException } from "../errors";

// createUser — representative Express/Nest-flavoured service action with an
// env-gate (process.env.SIGNUP_ENABLED), two early-return guards returning HTTP
// statuses, and a try/catch that logs then re-throws an HttpException(500).
export async function createUser(req: Request, res: Response) {
  if (!process.env.SIGNUP_ENABLED) {
    return res.status(503).json({ error: "signup disabled" });
  }

  if (req.body.email == null) {
    return res.status(400).json({ error: "email is required" });
  }

  try {
    const existing = await db.users.findByEmail(req.body.email);
    if (existing != null) {
      return res.status(409).json({ error: "email already in use" });
    }
    const user = await db.users.create(req.body);
    return res.status(201).json(user);
  } catch (e) {
    logger.error("createUser failed", e);
    throw new HttpException("create failed", 500);
  }
}
