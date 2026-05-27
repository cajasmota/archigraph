module Api
  class WidgetsController < ApplicationController
    def index
      render json: { widgets: [] }
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
  end
end
