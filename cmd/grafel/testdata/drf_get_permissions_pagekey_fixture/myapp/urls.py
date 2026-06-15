from rest_framework import routers

from . import views


router = routers.DefaultRouter()
router.register(r"jurisdictions", views.JurisdictionViewSet, basename="jurisdiction")


urlpatterns = router.urls
