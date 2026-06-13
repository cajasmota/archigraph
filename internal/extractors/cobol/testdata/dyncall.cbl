      ******************************************************************
      * dyncall.cbl — dynamic CALL <data-item> target resolution via   *
      *   MOVE-literal data-flow tracing (#5040).                      *
      ******************************************************************
       IDENTIFICATION DIVISION.
       PROGRAM-ID. DYNCALL.
       DATA DIVISION.
       WORKING-STORAGE SECTION.
       01  WS-PROGRAM      PIC X(8).
       01  WS-OTHER        PIC X(8).
       01  WS-COND         PIC X(8).
       01  WS-SRC          PIC X(8).
       PROCEDURE DIVISION.
       MAIN-PARA.
      *    Happy path: literal MOVE then dynamic CALL resolves to TAXCALC.
           MOVE 'TAXCALC' TO WS-PROGRAM.
           CALL WS-PROGRAM USING EMP-RATE WS-TOTAL.
      *    Last-write-wins: second literal MOVE overrides the first.
           MOVE 'OLDRATE' TO WS-OTHER.
           MOVE 'NEWRATE' TO WS-OTHER.
           CALL WS-OTHER USING WS-TOTAL.
           PERFORM TAINT-PARA.

       TAINT-PARA.
      *    Non-literal MOVE taints the item: stays unresolved (dynamic_ref).
           MOVE 'GOODPGM' TO WS-COND.
           MOVE WS-SRC TO WS-COND.
           CALL WS-COND USING WS-TOTAL.

       SCOPE-PARA.
      *    No MOVE in this paragraph: a binding must NOT leak across
      *    paragraph boundaries, so this CALL stays unresolved.
           CALL WS-PROGRAM USING WS-TOTAL.
      *    Literal CALL is unaffected by move-literal tracking.
           CALL 'AUDITLOG' USING WS-TOTAL.
