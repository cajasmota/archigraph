// NestJS backend-HTTP db_effect fixture (#2903).
// Controller methods delegate to a TypeORM repository; the sniffer
// attributes db_read / db_write to each handler method.
import { Controller, Get, Post, Body, Param } from "@nestjs/common";
import { Repository } from "typeorm";
import { User } from "./user.entity";

@Controller("users")
export class UsersController {
  constructor(private readonly repo: Repository<User>) {}

  async nestFindUser(id: string) {
    return await this.repo.findOne({ where: { id } });
  }

  async nestSaveUser(dto: Partial<User>) {
    return await this.repo.save(dto);
  }
}
