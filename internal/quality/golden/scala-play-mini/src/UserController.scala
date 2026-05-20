package controllers

import javax.inject._
import play.api._
import play.api.mvc._
import play.api.libs.json._
import scala.concurrent.{ExecutionContext, Future}
import models.{User, CreateUserRequest}
import services.UserService

case class User(id: Int, name: String, email: String)
case class CreateUserRequest(name: String, email: String)

@Singleton
class UserController @Inject()(
  val controllerComponents: ControllerComponents,
  userService: UserService
)(implicit ec: ExecutionContext) extends BaseController {

  def health() = Action {
    Ok(Json.obj("status" -> "ok"))
  }

  def listUsers() = Action.async {
    userService.findAll().map { users =>
      Ok(Json.toJson(users))
    }
  }

  def getUser(id: Int) = Action.async {
    userService.findById(id).map {
      case Some(user) => Ok(Json.toJson(user))
      case None       => NotFound(Json.obj("error" -> "not found"))
    }
  }

  def createUser() = Action.async(parse.json) { request =>
    request.body.validate[CreateUserRequest] match {
      case JsSuccess(req, _) =>
        userService.create(req).map { user =>
          Created(Json.toJson(user))
        }
      case JsError(errors) =>
        Future.successful(BadRequest(Json.obj("errors" -> JsError.toJson(errors))))
    }
  }

  def deleteUser(id: Int) = Action.async {
    userService.delete(id).map { deleted =>
      if (deleted) NoContent
      else NotFound(Json.obj("error" -> "not found"))
    }
  }
}
