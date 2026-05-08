from django.http import JsonResponse
from .models import Article


def article_list(request):
    return JsonResponse({"articles": list(Article.objects.values())})


def article_detail(request, pk):
    article = Article.objects.get(pk=pk)
    return JsonResponse({"id": article.id, "title": article.title})
