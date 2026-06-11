def sync(self, request):
    contacts = Contact.objects.filter(active=True)
    if request.data.get("force"):
        Contact.objects.create(name="seed")
    for contact in contacts:
        requests.post("https://api.example.com/notify", json={"id": contact.id})
    return Response({"ok": True})
