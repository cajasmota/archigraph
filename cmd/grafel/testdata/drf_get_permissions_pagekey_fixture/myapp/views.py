# Real-shape DRF ViewSet mirroring acme_core core/views/jurisdiction_viewset.py.
# get_permissions() branches on self.action and returns CustomPagePermissionCheck
# guards keyed by PERMISSION_PAGES["<KEY>"] PER ACTION. The resolved page key must
# reach the @action's synthesized endpoint as `auth_permissions` through the FULL
# index pipeline (not merely the engine-level ApplyDjangoDRFRoutes pass).
from rest_framework import viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

# Stand-ins for acme_core.settings.PERMISSION_PAGES and the custom guards.
PERMISSION_PAGES = {
    "JURISDICTIONS": "jurisdictions",
    "EMAIL_TEMPLATES": "email_templates",
}
DEFAULT_VIEWSET_ACTIONS = ["list", "retrieve", "create", "update", "partial_update", "destroy"]


class CustomPagePermissionCheck:
    def __init__(self, page):
        self.page = page


class CustomActionPermissionCheck:
    pass


class JurisdictionViewSet(viewsets.ModelViewSet):
    serializer_class = None
    http_method_names = ["get", "post", "put", "patch", "delete"]

    @action(methods=["get"], detail=False, url_path="inspection_types")
    def get_inspection_types(self, request, *arg, **kwargs):
        return Response({})

    @action(methods=["patch", "put"], detail=True)
    def email(self, request, pk, *args, **kwargs):
        return Response({})

    @action(methods=["get"], detail=False)
    def at_least_one_jurisdiction_has_maintenance_evaluation(self, request, *args, **kwargs):
        return Response({})

    def get_permissions(self):
        if self.action in DEFAULT_VIEWSET_ACTIONS:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["JURISDICTIONS"])]
        elif self.action in ["get_jurisdiction_setting", "get_inspection_types"]:
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["JURISDICTIONS"])]
        elif self.action == "email":
            permission_classes = [IsAuthenticated, CustomPagePermissionCheck(PERMISSION_PAGES["EMAIL_TEMPLATES"])]
        else:
            permission_classes = [IsAuthenticated, CustomActionPermissionCheck]
        return [permission() for permission in permission_classes]
