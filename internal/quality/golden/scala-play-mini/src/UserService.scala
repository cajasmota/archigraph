package services

import scala.concurrent.{ExecutionContext, Future}
import models.{User, CreateUserRequest}

trait UserService {
  def findAll(): Future[List[User]]
  def findById(id: Int): Future[Option[User]]
  def create(request: CreateUserRequest): Future[User]
  def delete(id: Int): Future[Boolean]
}

class UserServiceImpl()(implicit ec: ExecutionContext) extends UserService {

  private var users: List[User] = List(
    User(1, "Alice", "alice@example.com"),
    User(2, "Bob", "bob@example.com")
  )
  private var nextId: Int = 3

  def findAll(): Future[List[User]] = Future.successful(users)

  def findById(id: Int): Future[Option[User]] =
    Future.successful(users.find(_.id == id))

  def create(request: CreateUserRequest): Future[User] = {
    val user = User(nextId, request.name, request.email)
    nextId += 1
    users = users :+ user
    Future.successful(user)
  }

  def delete(id: Int): Future[Boolean] = {
    val before = users.length
    users = users.filterNot(_.id == id)
    Future.successful(users.length < before)
  }

  def transform(users: List[User]): List[User] =
    users.map(u => u.copy(name = u.name.toUpperCase))
      .filter(u => u.email.nonEmpty)
      .flatMap(u => List(u))
}
