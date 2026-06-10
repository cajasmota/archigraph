import time
import logging

from django.db import connection

logger = logging.getLogger("performance")


class PerformanceDebugMiddleware:
    def __init__(self, get_response):
        self.get_response = get_response

    def __call__(self, request):
        start = time.time()

        response = self.get_response(request)

        total_time = time.time() - start

        sql_time = 0
        query_count = 0

        for query in connection.queries:
            query_count += 1
            try:
                sql_time += float(query.get("time", 0))
            except Exception:
                pass

        print("\n" + "=" * 80)
        print(f"PATH: {request.path}")
        print(f"METHOD: {request.method}")
        print(f"STATUS: {response.status_code}")
        print(f"TOTAL TIME: {total_time:.2f}s")
        print(f"SQL TIME: {sql_time:.2f}s")
        print(f"QUERY COUNT: {query_count}")

        if total_time > 1:
            print("\nSLOW REQUEST DETECTED")

            sorted_queries = sorted(
                connection.queries,
                key=lambda x: float(x.get("time", 0)),
                reverse=True
            )

            for i, q in enumerate(sorted_queries[:10], start=1):
                print("\n" + "-" * 80)
                print(f"QUERY #{i}")
                print(f"TIME: {q.get('time')}s")
                print(q.get("sql"))

        print("=" * 80 + "\n")

        return response