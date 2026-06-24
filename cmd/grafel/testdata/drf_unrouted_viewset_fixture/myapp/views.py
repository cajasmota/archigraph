from rest_framework import viewsets


class WidgetViewSet(viewsets.ModelViewSet):
    """A DRF ModelViewSet that is NOT registered with any router.

    The router-expansion pass therefore emits NO router-expanded route
    entities for it — but the class declaration and its EXTENDS edge to
    ModelViewSet are indexed, so the inherited CRUD contract is still
    MRO-resolvable (the get_source path). This is the acme live-daemon
    shape that made grafel_effective_contract return empty.
    """

    queryset = []
    serializer_class = None
