class ThingsController < ApplicationController
  def index
    render json: { things: [] }
  end

  def show
    render json: { id: params[:id] }
  end

  def create
    render json: { stored: true }
  end

  def update
    render json: { updated: true }
  end

  def destroy
    render json: { deleted: true }
  end

  def new
    render json: { form: 'new' }
  end

  def edit
    render json: { form: 'edit' }
  end

  def search
    render json: { results: [] }
  end
end
