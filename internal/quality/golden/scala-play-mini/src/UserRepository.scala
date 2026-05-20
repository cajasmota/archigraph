package repositories

import scala.concurrent.{ExecutionContext, Future}
import models.User

trait UserRepository {
  def findAll(): Future[List[User]]
  def findById(id: Int): Future[Option[User]]
  def save(user: User): Future[User]
  def deleteById(id: Int): Future[Boolean]
}

class InMemoryUserRepository()(implicit ec: ExecutionContext) extends UserRepository {

  private var store: Map[Int, User] = Map.empty

  def findAll(): Future[List[User]] =
    Future.successful(store.values.toList)

  def findById(id: Int): Future[Option[User]] =
    Future.successful(store.get(id))

  def save(user: User): Future[User] = {
    store = store + (user.id -> user)
    Future.successful(user)
  }

  def deleteById(id: Int): Future[Boolean] = {
    val existed = store.contains(id)
    store = store - id
    Future.successful(existed)
  }

  def bulkSave(users: List[User]): Future[List[User]] = {
    val saved = users.map { user =>
      store = store + (user.id -> user)
      user
    }
    Future.successful(saved)
  }
}
