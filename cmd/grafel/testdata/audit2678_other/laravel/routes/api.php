<?php

use Illuminate\Support\Facades\Route;
use App\Http\Controllers\ThingController;

// Verb routes — handler lives in app/Http/Controllers/ThingController.php
Route::get('/things', [ThingController::class, 'index']);
Route::post('/things', [ThingController::class, 'store']);
Route::get('/things/{id}', [ThingController::class, 'show']);
