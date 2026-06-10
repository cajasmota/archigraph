from threading import local
from threading import current_thread
from django.utils.deprecation import MiddlewareMixin

_user = local()
_requests = {}


class CurrentUserMiddleware(MiddlewareMixin):

    @staticmethod
    def process_request(request):
        # _user.value = request.user
        _requests[current_thread()] = request


def get_current_user():
    try:
        # return _user.value
        t = current_thread()

        if t not in _requests:
            return None
        return _requests[t].user
    except AttributeError:
        return None
