// tRPC server fixture for #2693. Two routers compose into appRouter so
// the synthesizer must emit dotted-path endpoints (`users.list`,
// `users.byId`, `users.create`, `posts.list`, `posts.create`) rather than
// the leaf names alone.
//
// Each procedure leaf is an inline arrow expression — there is no named
// handler symbol to rebind to, so the source_line stamped on the
// synthetic is the `.query(` / `.mutation(` call's line. The integration
// test asserts that line precisely.

import { initTRPC } from "@trpc/server";
import { z } from "zod";

const t = initTRPC.create();
const publicProcedure = t.procedure;
const router = t.router;

const userRouter = router({
  list: publicProcedure.query(({ ctx }) => {
    return [{ id: "u1" }];
  }),
  byId: publicProcedure
    .input(z.object({ id: z.string() }))
    .query(({ input }) => {
      return { id: input.id };
    }),
  create: publicProcedure
    .input(z.object({ name: z.string() }))
    .mutation(({ input }) => {
      return { id: "u_new", name: input.name };
    }),
});

const postsRouter = router({
  list: publicProcedure.query(({ ctx }) => {
    return [{ id: "p1" }];
  }),
  create: publicProcedure
    .input(z.object({ title: z.string() }))
    .mutation(({ input }) => {
      return { id: "p_new", title: input.title };
    }),
});

export const appRouter = router({
  users: userRouter,
  posts: postsRouter,
});

export type AppRouter = typeof appRouter;
