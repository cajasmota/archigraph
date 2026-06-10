from django.http import JsonResponse
from django.contrib.auth.models import AnonymousUser
from rest_framework.authtoken.models import Token
from django.utils.deprecation import MiddlewareMixin
from rest_framework.authentication import TokenAuthentication
from core.models import UserGroupRole

INSPECTOR_ROLE_ID = 4

class TokenAuthenticationMiddleware(MiddlewareMixin):
    """
    Middleware to authenticate users using the token in the Authorization header.
    """

    def process_request(self, request):
        token_key = request.headers.get("Authorization")

        if token_key and token_key.startswith("Bearer ") == False:
            try:
                # Fetch user through token with a minimal field-set to avoid
                # decrypting heavy encrypted columns (e.g. dob_password) on
                # every request.
                token = (
                    Token.objects.select_related("user")
                    .only(
                        "key",
                        "user__id",
                        "user__email",
                        "user__first_name",
                        "user__last_name",
                        "user__is_superuser",
                        "user__is_staff",
                        "user__status",
                    )
                    .get(key=token_key)
                )
                user = token.user
                request.user = user  # Attach user to request

                request._force_auth_user = user
                request.authenticators = [TokenAuthentication()]
                request.successful_authenticator = TokenAuthentication()
                request.auth = (user, token)
                inspector_membership = (
                    UserGroupRole.objects.select_related("group")
                    .filter(user=user, role_id=INSPECTOR_ROLE_ID)
                    .order_by("id")
                    .first()
                )
                request.group = (
                    inspector_membership.group
                    if inspector_membership
                    else user.groups.only("id", "name").first()
                )
                request.jurisdiction = (
                    request.group.jurisdictions.first() if request.group else None
                )
            except Token.DoesNotExist:
                return JsonResponse({"error": "Invalid token"}, status=401)
        # else:
            # request.user = AnonymousUser()  # Set as an unauthenticated user
