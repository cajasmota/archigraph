// groovy-grails-mini fixture — Grails RestfulController with GORM domain
// Used by TestGroovyGrailsDynamic to verify groovyDynamicPatterns (issue #44).
package myapp

import grails.rest.RestfulController
import grails.validation.ValidationException
import org.springframework.http.HttpStatus

class PostController extends RestfulController<Post> {

    static responseFormats = ['json', 'xml']
    static allowedMethods = [save: "POST", update: "PUT", delete: "DELETE"]

    PostService postService

    PostController() {
        super(Post)
    }

    @Override
    def index(Integer max) {
        respond postService.list(params)
    }

    @Override
    def show() {
        respond findResource(params.id)
    }

    @Override
    def save() {
        def post = new Post(params)
        bindData(post, request)

        try {
            postService.save(post)
        } catch (ValidationException e) {
            respond post.errors, view: 'create'
            return
        }

        respond post, [status: HttpStatus.CREATED, view: 'show']
    }

    @Override
    def update() {
        def post = findResource(params.id)
        if (post == null) {
            transactionStatus.setRollbackOnly()
            render status: HttpStatus.NOT_FOUND
            return
        }

        bindData(post, request)
        postService.save(post)

        respond post, view: 'show'
    }

    @Override
    def delete() {
        def post = findResource(params.id)
        if (post == null) {
            transactionStatus.setRollbackOnly()
            render status: HttpStatus.NOT_FOUND
            return
        }

        postService.delete(post.id)
        render status: HttpStatus.NO_CONTENT
    }
}

class Post {
    String title
    String body

    static constraints = {
        title blank: false
        body blank: false
    }

    static int countPublished() {
        return count()
    }
}
