<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;

class ThingController extends Controller
{
    public function index(Request $request)
    {
        return ['things' => []];
    }

    public function store(Request $request)
    {
        return ['stored' => true];
    }

    public function show($id)
    {
        return ['id' => $id];
    }
}
